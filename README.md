# ACM Secrets Controller

Syncs your TLS secrets to ACM.

## Instructions

The ACM certificate has to exist before syncing to it. This is due only being able to look up ACM certificates by arn.
Create an empty ACM certificate:
```
openssl genrsa 2048 > tls.key
openssl req -new -x509 -subj "/CN=example.com" -nodes -sha1 -days 3650 -key tls.key > tls.crt
CERT=$(aws acm import-certificate --certificate fileb://tls.crt --private-key fileb://tls.key)
echo $CERT
```
(save this value for later)

Install nginx-ingress
```
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm install nginx ingress-nginx/ingress-nginx
```

Install cert-manager
```
kubectl create namespace cert-manager

helm repo add jetstack https://charts.jetstack.io


helm install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --version v1.0.3 \
  --set installCRDs=true
```

Create issuer
```
EMAIL="my.email@gmail.com"

cat << EOF > issuer.yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-staging
  namespace: cert-manager-staging
spec:
  acme:
    # You must replace this email address with your own.
    # Let's Encrypt will use this to contact you about expiring
    # certificates, and issues related to your account.
    email: $EMAIL
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      # Secret resource that will be used to store the account's private key.
      name: example-issuer-account-key
    # Add a single challenge solver, HTTP01 using nginx
    solvers:
    - http01:
        ingress:
          class: nginx
EOF

kubectl create namespace cert-manager-staging
kubectl apply -f issuer.yaml
```

Finally, create a certificate CRD. `.sec.secretName` name must match the ID of the certificate ARN:
```
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: 1629381a-05d6-4ec9-8b2f-ea7db1a29b58
  namespace: cert-manager-test
  annotations:
    x: "y"
spec:
  dnsNames:
    - example.com
    - test.com
  secretName: 1629381a-05d6-4ec9-8b2f-ea7db1a29b58
  issuerRef:
    name: letsencrypt-staging
    kind: ClusterIssuer
```
