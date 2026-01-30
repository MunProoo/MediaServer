package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type sslgenInfo struct {
	certpemfilepath string
	keypemfilepath  string
}

func initMediaSSLgen(WCServerAddress string) (sslgen *sslgenInfo) {
	curPath, err := filepath.Abs(filepath.Dir(os.Args[0])) // 실행 경로

	if _, err = os.Stat(curPath + "/sslgen"); os.IsNotExist(err) { // 폴더 생성
		if err = os.Mkdir(curPath+"/sslgen", os.FileMode(0755)); err != nil {
			log.Printf("[ERROR] [sslgen] [initMediaSSLgen] Failed to create folder: err=%v", err)
			return
		}
	}
	tmpPath := curPath + "/sslgen"

	max := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, max)
	subject := pkix.Name{
		Organization:       []string{"MJY Co."},
		OrganizationalUnit: []string{"RND"},
		CommonName:         "Alpeta project",
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP(WCServerAddress)},
	}

	pk, _ := rsa.GenerateKey(rand.Reader, 2048)
	derBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &pk.PublicKey, pk)
	sslgen = &sslgenInfo{}
	sslgen.certpemfilepath = tmpPath + "/AlpetaMediaCertificate.pem"
	if _, err := os.Stat(sslgen.certpemfilepath); os.IsNotExist(err) { // 없으면 생성
		certOut, _ := os.Create(sslgen.certpemfilepath)
		pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
		certOut.Close()
	}
	sslgen.keypemfilepath = tmpPath + "/AlpetaMediaPrivateKey.pem"
	if _, err := os.Stat(sslgen.keypemfilepath); os.IsNotExist(err) { // 없으면 생성
		keyOut, _ := os.Create(sslgen.keypemfilepath)
		pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)})
		keyOut.Close()
	}

	return
}
