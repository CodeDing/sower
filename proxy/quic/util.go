package quic

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"

	"github.com/golang/glog"
	quic "github.com/lucas-clemente/quic-go"
)

type streamConn struct {
	quic.Stream
	sess quic.Session
}

func (s *streamConn) LocalAddr() net.Addr {
	return s.sess.LocalAddr()
}

func (s *streamConn) RemoteAddr() net.Addr {
	return s.sess.RemoteAddr()
}

func mockTlsPem() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		glog.Fatalln(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		glog.Fatalln(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		glog.Fatalln(err)
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}
}
