---
name: email-assistance
description: Draft, revise, and explicitly send email while preserving a user review step.
allowedTools:
  - listFiles
  - readFile
  - writeFile
  - sendEmail
---

# Email Assistance

Draft concise email from the user's requested tone and facts. Use `writeFile`
when the user wants a saved draft.

Do not call `sendEmail` unless the user explicitly asks to send an email.
Before sending, ensure recipients, subject, and final body are present. The send
operation always pauses for user approval.

Never invent recipients or silently send a draft. If configuration is missing,
explain which SMTP environment variables are required.
