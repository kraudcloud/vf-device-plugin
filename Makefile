VERSION=9


deploy: docker

	sed -e 's/xxxVERSIONxxx/$(VERSION)/' < manifest/k8s.yml | kubectl apply -f -

docker: vf-device-plugin
	docker build . -t ctr.0x.pt/kraud/vf-device-plugin:$(VERSION)
	docker push ctr.0x.pt/kraud/vf-device-plugin:$(VERSION)
	

vf-device-plugin: .PHONY
	CGO_ENABLED=0 go build -o vf-device-plugin

.PHONY:
