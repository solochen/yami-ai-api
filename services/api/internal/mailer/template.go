package mailer

import (
	"fmt"
	"html"
	"strings"
)

// 邮件模板使用的品牌色，取自 apps/web 的设计 token
// （tailwind.config.ts 的 primary/dark/surface 与 globals.css 的 mcdl token）。
const (
	emailAccent    = "#FF8711" // 主橙
	emailAccentDim = "#F2C18D" // 浅橙
	emailInk       = "#160B03" // 暖炭（标题）
	emailSurface   = "#F3EEE9" // 暖米（验证码底）
	emailCanvas    = "#EEF1F6" // 页面底色
	emailCard      = "#FFFFFF"
	emailHairline  = "#EDE5DD"
	emailBodyText  = "#6B5D53"
	emailMutedText = "#9C8D82"

	emailFontStack = "-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Hiragino Sans GB','Microsoft YaHei',Roboto,Helvetica,Arial,sans-serif"
	emailMonoStack = "'SFMono-Regular',Consolas,'Liberation Mono',Menlo,monospace"
)

// OTPEmailData 是登录验证码邮件的渲染入参。
type OTPEmailData struct {
	SiteName string
	Tagline  string
	LogoURL  string
	SiteURL  string
	Code     string
	Minutes  int
}

// RenderOTPEmail 生成验证码邮件的 HTML 与纯文本两个版本。
// 纯文本版本不只是降级兜底：纯文本占比高的邮件更不容易被内容过滤器判为营销邮件。
func RenderOTPEmail(d OTPEmailData) (htmlBody, textBody string) {
	name := strings.TrimSpace(d.SiteName)
	if name == "" {
		name = "Orbit-E AI"
	}
	minutes := d.Minutes
	if minutes <= 0 {
		minutes = 10
	}
	esc := html.EscapeString
	siteURL := strings.TrimSpace(d.SiteURL)

	logo := ""
	if strings.TrimSpace(d.LogoURL) != "" {
		logo = fmt.Sprintf(`<img src="%s" width="40" height="40" alt="%s" style="display:block;width:40px;height:40px;border-radius:12px;border:0;outline:none;text-decoration:none;">`,
			esc(strings.TrimSpace(d.LogoURL)), esc(name))
	}

	tagline := ""
	if t := strings.TrimSpace(d.Tagline); t != "" {
		tagline = fmt.Sprintf(`<div style="padding-top:3px;font-size:12px;line-height:1.4;color:%s;">%s</div>`,
			emailMutedText, esc(t))
	}

	footerSite := ""
	if siteURL != "" {
		footerSite = fmt.Sprintf(`<a href="%s" style="color:%s;text-decoration:none;">%s</a>`,
			esc(siteURL), emailMutedText, esc(siteURL))
	}

	htmlBody = strings.NewReplacer(
		"{{PREHEADER}}", esc(fmt.Sprintf("您的 %s 登录验证码是 %s，%d 分钟内有效。", name, d.Code, minutes)),
		"{{LOGO}}", logo,
		"{{SITE_NAME}}", esc(name),
		"{{TAGLINE}}", tagline,
		"{{CODE}}", esc(d.Code),
		"{{MINUTES}}", fmt.Sprintf("%d", minutes),
		"{{FOOTER_SITE}}", footerSite,
		"{{ACCENT}}", emailAccent,
		"{{ACCENT_DIM}}", emailAccentDim,
		"{{INK}}", emailInk,
		"{{SURFACE}}", emailSurface,
		"{{CANVAS}}", emailCanvas,
		"{{CARD}}", emailCard,
		"{{HAIRLINE}}", emailHairline,
		"{{BODY_TEXT}}", emailBodyText,
		"{{MUTED_TEXT}}", emailMutedText,
		"{{FONT}}", emailFontStack,
		"{{MONO}}", emailMonoStack,
	).Replace(otpEmailHTML)

	var b strings.Builder
	fmt.Fprintf(&b, "您好！\r\n\r\n您正在登录 %s，请在验证页面输入以下验证码完成身份验证：\r\n\r\n    %s\r\n\r\n验证码 %d 分钟内有效，请勿泄露给他人。\r\n\r\n如果这不是您本人的操作，请忽略此邮件，您的账号仍然安全。\r\n\r\n— %s\r\n",
		name, d.Code, minutes, name)
	if siteURL != "" {
		fmt.Fprintf(&b, "%s\r\n", siteURL)
	}
	b.WriteString("\r\n此邮件由系统自动发送，请勿直接回复。")
	textBody = b.String()

	return htmlBody, textBody
}

// 邮件客户端普遍不支持 <style> 与外链样式表，因此全部使用行内样式；
// 布局用 table 而非 flex/grid，以保证 Outlook 桌面端渲染一致。
const otpEmailHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>{{SITE_NAME}} 登录验证码</title>
<style>
/* 行内样式是各邮件客户端的基线；这段只做渐进增强，被剥离也不影响布局。 */
@media only screen and (max-width:600px){
.oe-pad{padding-left:20px !important;padding-right:20px !important;}
.oe-code{font-size:30px !important;}
}
</style>
</head>
<body style="margin:0;padding:0;background:{{CANVAS}};">
<div style="display:none;font-size:1px;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;color:{{CANVAS}};">{{PREHEADER}}</div>
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:{{CANVAS}};">
<tr>
<td align="center" style="padding:36px 16px;">

<table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="width:100%;max-width:600px;background:{{CARD}};border:1px solid {{HAIRLINE}};border-radius:16px;">

<tr>
<td style="height:4px;line-height:4px;font-size:0;background:{{ACCENT}};border-radius:15px 15px 0 0;">&nbsp;</td>
</tr>

<tr>
<td class="oe-pad" style="padding:26px 32px 20px 32px;">
<table role="presentation" cellpadding="0" cellspacing="0" border="0">
<tr>
<td style="padding-right:12px;vertical-align:middle;">{{LOGO}}</td>
<td style="vertical-align:middle;font-family:{{FONT}};">
<div style="font-size:17px;font-weight:700;color:{{INK}};letter-spacing:0.01em;">{{SITE_NAME}}</div>
{{TAGLINE}}
</td>
</tr>
</table>
</td>
</tr>

<tr>
<td class="oe-pad" style="padding:0 32px;"><div style="height:1px;line-height:1px;font-size:0;background:{{HAIRLINE}};">&nbsp;</div></td>
</tr>

<tr>
<td class="oe-pad" style="padding:26px 32px 0 32px;font-family:{{FONT}};">
<div style="font-size:21px;font-weight:700;color:{{INK}};letter-spacing:0.01em;">登录验证码</div>
<div style="padding-top:10px;font-size:14px;line-height:1.75;color:{{BODY_TEXT}};">
您好！您正在登录 {{SITE_NAME}}，请在验证页面输入下方验证码完成身份验证。
</div>
</td>
</tr>

<tr>
<td class="oe-pad" style="padding:22px 32px 0 32px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:{{SURFACE}};border-radius:12px;">
<tr>
<td align="center" style="padding:20px 16px;">
<div class="oe-code" style="font-family:{{MONO}};font-size:34px;font-weight:700;letter-spacing:0.3em;padding-left:0.3em;color:{{ACCENT}};">{{CODE}}</div>
<div style="padding-top:8px;font-family:{{FONT}};font-size:12px;color:{{MUTED_TEXT}};">验证码 {{MINUTES}} 分钟内有效</div>
</td>
</tr>
</table>
</td>
</tr>

<tr>
<td class="oe-pad" style="padding:18px 32px 0 32px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#FCFAF7;border-left:3px solid {{ACCENT_DIM}};border-radius:8px;">
<tr>
<td style="padding:13px 16px;font-family:{{FONT}};font-size:12.5px;line-height:1.7;color:{{BODY_TEXT}};">
出于安全考虑，请勿将此验证码转发给任何人。如果这不是您本人的操作，请忽略此邮件，您的账号仍然安全。
</td>
</tr>
</table>
</td>
</tr>

<tr>
<td class="oe-pad" style="padding:26px 32px 28px 32px;font-family:{{FONT}};font-size:12px;line-height:1.7;color:{{MUTED_TEXT}};">
此邮件由系统自动发送，请勿直接回复。<br>
{{FOOTER_SITE}}
</td>
</tr>

</table>

</td>
</tr>
</table>
</body>
</html>`
