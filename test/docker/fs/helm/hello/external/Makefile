default: all

all: helm test

helm:
	helm install -n external-hello ./chart

test:
	sport=$(shell kubectl get svc -o go-template='{{range .items}}{{if eq .metadata.name "external-hello-service"}}{{range.spec.ports}}{{if .nodePort}}{{.nodePort}}{{"\n"}}{{end}}{{end}}{{end}}{{end}}') && \
		curl -sSL http://localhost:$$sport

stop:
	-helm delete --purge external-hello

package:
	helm package ./external-hello

.PHONY: default all
