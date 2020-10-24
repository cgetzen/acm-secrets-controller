package main

import (
	log "github.com/Sirupsen/logrus"
	core_v1 "k8s.io/api/core/v1"
	"github.com/aws/aws-sdk-go/service/acm"
	"fmt"
	"regexp"
	"strings"
)

var CERT_END string = "-----END CERTIFICATE-----"

// Handler interface contains the methods that are required
type Handler interface {
	Init() error
	ObjectCreated(obj interface{})
	ObjectDeleted(obj interface{})
	ObjectUpdated(objOld, objNew interface{})
}

// SecretHandler is a sample implementation of Handler
type SecretHandler struct{
	AccountID string
	Client *acm.ACM
}

// Init handles any handler initialization
func (t *SecretHandler) Init() error {
	log.Info("SecretHandler.Init")
	return nil
}

func difference(a []string, b []string) []string {
    cache := make(map[string]bool, len(b))
    for _, s := range b {
        cache[s] = true
    }
    var diff []string
    for _, s := range a {
        if _, found := cache[s]; !found {
            diff = append(diff, s)
        }
    }
    return diff
}

// ObjectCreated is called when an object is created
func (t *SecretHandler) ObjectCreated(obj interface{}) {
	secret := obj.(*core_v1.Secret)

	if secret.Type != "kubernetes.io/tls" {
		log.Info("Ignoring - not type kubernetes.io/tls\n")
		return
	}
	if matched, _ := regexp.MatchString("[A-Za-z0-9]*-[A-Za-z0-9]*-[A-Za-z0-9]*-[A-Za-z0-9]*-[A-Za-z0-9]*", secret.Name); !matched {
		log.Info("Ignoring - name does not match ID regex\n")
		return
	}

	arn := fmt.Sprintf("arn:aws:acm:us-west-2:%s:certificate/%s", t.AccountID, secret.Name)
	result, err := t.Client.DescribeCertificate(&acm.DescribeCertificateInput{
		CertificateArn: &arn,
	})
	if err != nil {
		log.Infof(fmt.Sprintf("%s\n", err.Error()))
		return
	}
	secretSan := strings.Split(secret.Annotations["cert-manager.io/alt-names"], ",")
	certSan := []string{}
	for _, name := range result.Certificate.SubjectAlternativeNames {
		certSan = append(certSan, *name)
	}
	if len(difference(certSan, secretSan)) > 1 {
		log.Info(certSan)
		log.Info(secretSan)
		log.Info(difference(certSan, secretSan))
		log.Infof("This action is unsafe (overwrites > 1 SAN)\n")
		return
	}
	if len(difference(secretSan, certSan)) == 0 {
		log.Infof("No changes needed\n")
		return
	}

	cert := string(secret.Data["tls.crt"])
	key := string(secret.Data["tls.key"])

	i := strings.Index(string(cert), CERT_END) + len(CERT_END)

	output, err := t.Client.ImportCertificate(&acm.ImportCertificateInput{
		Certificate: []byte(cert[0:i]),
		CertificateArn: &arn,
		CertificateChain: []byte(cert[i+1:len(cert)]),
		PrivateKey: []byte(key),
	})
	if err != nil {
		log.Infof("Import error: %s", err.Error())
		return
	}
	log.Infof("Written %s\n", output.CertificateArn)
}

// ObjectDeleted is called when an object is deleted
func (t *SecretHandler) ObjectDeleted(obj interface{}) {
	log.Info("SecretHandler.ObjectDeleted")
}

// ObjectUpdated is called when an object is updated
func (t *SecretHandler) ObjectUpdated(objOld, objNew interface{}) {
	log.Info("SecretHandler.ObjectUpdated")
}
