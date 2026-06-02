package definitions

import (
	"context"
	"fmt"
	"net/smtp"
	"os"
	"strings"

	"github.com/CaowardlyLion/OpenHome/internal/permissions"
	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func SendEmail() tools.Definition {
	return tools.Definition{
		Name: "sendEmail", Advanced: true, Effect: tools.EffectMutate,
		Description: "Send an email through configured SMTP only after explicit user approval. Use only when the user explicitly asks to send, not merely draft, an email.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false, "required": []string{"to", "subject", "body", "reason"},
			"properties": map[string]any{
				"to":      map[string]any{"type": "array", "minItems": 1, "maxItems": 20, "items": map[string]any{"type": "string"}},
				"subject": map[string]any{"type": "string"},
				"body":    map[string]any{"type": "string"},
				"reason":  map[string]any{"type": "string"},
			},
		},
		Execute: func(ctx context.Context, toolContext tools.Context, args map[string]any) (any, error) {
			to, err := stringSliceArg(args, "to")
			if err != nil || len(to) == 0 {
				return nil, fmt.Errorf("argument %q must contain at least one recipient", "to")
			}
			subject, err := stringArg(args, "subject")
			if err != nil {
				return nil, err
			}
			body, err := stringArg(args, "body")
			if err != nil {
				return nil, err
			}
			if len(body) > 256<<10 {
				return nil, fmt.Errorf("email body exceeds 256 KB")
			}
			reason, err := stringArg(args, "reason")
			if err != nil {
				return nil, err
			}
			from, address := os.Getenv("OPENHOME_SMTP_FROM"), os.Getenv("OPENHOME_SMTP_ADDR")
			if from == "" || address == "" {
				return nil, fmt.Errorf("sendEmail requires OPENHOME_SMTP_FROM and OPENHOME_SMTP_ADDR")
			}
			if !validEmailHeader(from) || !validEmailHeader(subject) {
				return nil, fmt.Errorf("email headers must not contain line breaks")
			}
			for _, recipient := range to {
				if recipient == "" || !validEmailHeader(recipient) {
					return nil, fmt.Errorf("email recipients must be non-empty and must not contain line breaks")
				}
			}
			if toolContext.Permissions == nil || toolContext.Permissions.Authorize(ctx, permissions.Request{
				Tool: "sendEmail", Reason: reason, Target: strings.Join(to, ", "), SimilarKey: strings.Join(to, ","),
				Details: "Subject: " + subject, Elevated: true,
			}) == permissions.Deny {
				return denied("sendEmail"), nil
			}
			host := strings.Split(address, ":")[0]
			var auth smtp.Auth
			if user := os.Getenv("OPENHOME_SMTP_USER"); user != "" {
				auth = smtp.PlainAuth("", user, os.Getenv("OPENHOME_SMTP_PASSWORD"), host)
			}
			message := []byte("From: " + from + "\r\nTo: " + strings.Join(to, ", ") + "\r\nSubject: " + subject + "\r\n\r\n" + body)
			if err := smtp.SendMail(address, auth, from, to, message); err != nil {
				return nil, err
			}
			return map[string]any{"sent": true, "to": to, "subject": subject}, nil
		},
	}
}

func validEmailHeader(value string) bool {
	return !strings.ContainsAny(value, "\r\n")
}
