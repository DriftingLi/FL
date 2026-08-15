package service

// sendSMTPS（SMTP 465 隐式 SSL）单元测试：
// 本地 TLS SMTP 假服务器走完整 SMTP 会话（EHLO→AUTH→MAIL→RCPT→DATA→QUIT），
// 锁定邮件内容送达与连接失败路径。
import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

// newTestTLSConfig 生成测试用自签证书与 TLS 配置（客户端跳过验证）。
func newTestTLSConfig(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	serverCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	clientCfg := &tls.Config{InsecureSkipVerify: true, ServerName: "127.0.0.1", MinVersion: tls.VersionTLS12}
	return serverCfg, clientCfg
}

// fakeSMTPS 本地 TLS SMTP 服务器：接受一个连接、按会话对话、记录 DATA 内容。
func fakeSMTPS(t *testing.T, serverCfg *tls.Config) (addr string, received chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsLn := tls.NewListener(ln, serverCfg)
	received = make(chan []byte, 1)
	go func() {
		conn, err := tlsLn.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		br := bufio.NewReader(conn)
		write := func(s string) {
			_, _ = conn.Write([]byte(s))
		}
		write("220 test ESMTP\r\n")
		var data []byte
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.TrimRight(line, "\r\n")
			upper := strings.ToUpper(cmd)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				write("250-test\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(upper, "AUTH"):
				write("235 2.7.0 ok\r\n")
			case strings.HasPrefix(upper, "MAIL FROM"):
				write("250 ok\r\n")
			case strings.HasPrefix(upper, "RCPT TO"):
				write("250 ok\r\n")
			case upper == "DATA":
				write("354 go\r\n")
				for {
					dl, err := br.ReadString('\n')
					if err != nil {
						return
					}
					if dl == ".\r\n" || dl == ".\n" {
						break
					}
					data = append(data, dl...)
				}
				write("250 ok\r\n")
			case upper == "QUIT":
				write("221 bye\r\n")
				received <- data
				return
			default:
				write("250 ok\r\n")
			}
		}
	}()
	t.Cleanup(func() { _ = tlsLn.Close() })
	return ln.Addr().String(), received
}

// TestSendSMTPS_Success 完整会话送达邮件内容（含头部与正文）。
func TestSendSMTPS_Success(t *testing.T) {
	serverCfg, clientCfg := newTestTLSConfig(t)
	addr, received := fakeSMTPS(t, serverCfg)

	msg := []byte("From: A <a@x.com>\r\nTo: <b@x.com>\r\nSubject: 测试\r\n\r\n正文内容\r\n")
	auth := smtp.PlainAuth("", "u", "p", "127.0.0.1")
	err := sendSMTPS(addr, "127.0.0.1", auth, "a@x.com", "b@x.com", msg, clientCfg)
	if err != nil {
		t.Fatalf("sendSMTPS 失败: %v", err)
	}

	select {
	case got := <-received:
		if !strings.Contains(string(got), "Subject: 测试") || !strings.Contains(string(got), "正文内容") {
			t.Fatalf("服务器收到的邮件缺少内容:\n%s", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("超时未收到邮件内容")
	}
}

// TestSendSMTPS_ConnectFail 连接失败返回错误。
func TestSendSMTPS_ConnectFail(t *testing.T) {
	_, clientCfg := newTestTLSConfig(t)
	// 未监听端口：连接被拒绝
	auth := smtp.PlainAuth("", "u", "p", "127.0.0.1")
	err := sendSMTPS("127.0.0.1:1", "127.0.0.1", auth, "a@x.com", "b@x.com", []byte("x"), clientCfg)
	if err == nil {
		t.Fatal("连接失败应返回错误")
	}
}
