# ==================== Running ====================

.PHONY: build run unit integration genmock genproto gencert
build:
	docker build -t local/jobrunner .

.PHONY: run
run:
	docker run --rm --name jobrunner \
 		--volume ${CURDIR}/certs:/etc/ssl/certs \
 		--volume ${CURDIR}/config:/etc/jobrunner \
 		-p 9090:9090 \
 		local/jobrunner -config /etc/jobrunner/config.yaml

# ==================== Testing ====================

unit:
	go test -count=1 ./...

integration:
	go test ./cmd... --tags=integration -count=1

# ==================== Code Gen ===================

.PHONY: genmock
genmock:
	go generate ./...

.PHONY: genproto
genproto:
	./proto/genproto.sh

# ==================== Certs =====================

.PHONY: gencert
CERT_PATH=certs/
gencert:
	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
    		-initca config/cert/ca-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare ca

	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=config/cert/ca-config.json \
		-profile=server \
		config/cert/server-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare server

	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=config/cert/ca-config.json \
		-profile=client \
		config/cert/client-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare client

	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=config/cert/ca-config.json \
		-profile=client \
		-cn="root" \
		config/cert/client-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare root-client

	mv *.pem *.csr ${CERT_PATH}


.PHONY: testcert
TEST_CERT_PATH=testdata/
testcert:
	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
    		-initca config/cert/ca-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare ca

	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=config/cert/ca-config.json \
		-profile=server \
		config/cert/server-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare server

	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=config/cert/ca-config.json \
		-profile=client \
		config/cert/client-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare client

	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=config/cert/ca-config.json \
		-profile=client \
		-cn="root" \
		config/cert/client-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare root-client

	go run github.com/cloudflare/cfssl/cmd/cfssl gencert \
		-ca=ca.pem \
		-ca-key=ca-key.pem \
		-config=config/cert/ca-config.json \
		-profile=client \
		-cn="nobody" \
		config/cert/client-csr.json | go run github.com/cloudflare/cfssl/cmd/cfssljson -bare nobody-client

	mv *.pem *.csr ${TEST_CERT_PATH}
