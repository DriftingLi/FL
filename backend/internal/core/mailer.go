// 本文件：邮件发送器单点（spec #449 决定 15）——验证码通道与招聘域（联系方式交换/投递通知）共用。
//
// P2 波 3a（auth 域搬包）起本文件是**留驻面**：MailSender 一族不进 internal/auth，
// 因为 contact_service.go / job_application_service.go / job_report_service.go 三个留驻消费者
// 仍在这里用它；把它搬进域包会逼 internal/core import internal/auth，与
// 「域包 → internal/core 单向边」构成 import cycle。auth 侧一律写 core.MailSender /
// core.NewMailSender。
package core

import (
	"crypto/tls"
	"fmt"
	"net/smtp"

	"go.uber.org/zap"

	"forklift-training/internal/config"
)

// MailSender 邮件发送接口（SMTP 生产实现 / 日志降级实现 / 测试替身）。
type MailSender interface {
	Send(to, subject, body string) error
}

// SMTPMailSender 通过 SMTP 发送邮件。
type SMTPMailSender struct {
	cfg config.SMTPConfig
}

// Send 发送一封 UTF-8 纯文本邮件。
// 端口 465 走隐式 SSL（sendSMTPS）；其余端口走 smtp.SendMail（587 为 STARTTLS）。
// 部分网络环境（透明 SMTP 代理/防火墙）会阻断 587 的 STARTTLS 握手，465 可绕过。
func (s SMTPMailSender) Send(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	from := s.cfg.From
	msg := fmt.Sprintf(
		"From: %s <%s>\r\nTo: <%s>\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.cfg.FromName, from, to, subject, body,
	)

	if s.cfg.Port == 465 {
		return sendSMTPS(addr, s.cfg.Host, auth, from, to, []byte(msg), nil)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// sendSMTPS 通过隐式 SSL（465）发送邮件：先建立 TLS 连接，再走完整 SMTP 会话。
// 独立成函数以便用本地 TLS SMTP 假服务器做单元测试；tlsCfg 为 nil 时使用默认配置。
func sendSMTPS(addr, serverName string, auth smtp.Auth, from, to string, msg []byte, tlsCfg *tls.Config) error {
	if tlsCfg == nil {
		tlsCfg = &tls.Config{ServerName: serverName}
	}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, serverName)
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// LogMailSender 开发环境降级实现：验证码写入服务日志（未配置 SMTP 时便于本地验证）。
type LogMailSender struct {
	logger *zap.Logger
}

// Send 将邮件内容写入日志。
func (s LogMailSender) Send(to, subject, body string) error {
	s.logger.Info("邮件发送（开发环境降级为日志）", zap.String("to", to), zap.String("subject", subject), zap.String("body", body))
	return nil
}

// NewMailSender 邮件发送器工厂（spec #449 决定 15 的单点）：
// 生产配置了 SMTP → SMTP 实现；未配置且是生产 → nil（调用方降级日志）；
// 开发/测试 → 日志降级实现。验证码通道与招聘域（联系方式交换/投递通知）共用此单点。
func NewMailSender(smtpCfg config.SMTPConfig, isProd bool, logger *zap.Logger) MailSender {
	if smtpCfg.Host != "" && smtpCfg.From != "" {
		return SMTPMailSender{cfg: smtpCfg}
	}
	if isProd {
		return nil
	}
	return LogMailSender{logger: logger}
}
