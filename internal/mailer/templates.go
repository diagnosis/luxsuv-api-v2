package mailer

const WelcomeEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome</title>
</head>
<body style="margin: 0; padding: 0; font-family: Arial, sans-serif; background-color: #f4f4f4;">
    <table cellpadding="0" cellspacing="0" width="100%" style="background-color: #f4f4f4; padding: 20px;">
        <tr>
            <td align="center">
                <table cellpadding="0" cellspacing="0" width="600" style="background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
                    <tr>
                        <td style="padding: 40px 30px; text-align: center; background-color: #2563eb;">
                            <h1 style="margin: 0; color: #ffffff; font-size: 28px;">Welcome to Lux SUV</h1>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding: 40px 30px;">
                            <h2 style="margin: 0 0 20px 0; color: #333333; font-size: 24px;">Hello {{.Name}}!</h2>
                            <p style="margin: 0 0 20px 0; color: #666666; font-size: 16px; line-height: 1.6;">
                                Thank you for joining Lux SUV. We're excited to have you on board!
                            </p>
                            <p style="margin: 0 0 20px 0; color: #666666; font-size: 16px; line-height: 1.6;">
                                Your account has been successfully created with the email: <strong>{{.Email}}</strong>
                            </p>
                            <p style="margin: 0; color: #666666; font-size: 16px; line-height: 1.6;">
                                If you have any questions or need assistance, feel free to reach out to our support team.
                            </p>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding: 30px; text-align: center; background-color: #f8f9fa; border-top: 1px solid #e9ecef;">
                            <p style="margin: 0; color: #999999; font-size: 14px;">
                                &copy; 2025 Lux SUV. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`

const PasswordResetTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Password Reset</title>
</head>
<body style="margin: 0; padding: 0; font-family: Arial, sans-serif; background-color: #f4f4f4;">
    <table cellpadding="0" cellspacing="0" width="100%" style="background-color: #f4f4f4; padding: 20px;">
        <tr>
            <td align="center">
                <table cellpadding="0" cellspacing="0" width="600" style="background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
                    <tr>
                        <td style="padding: 40px 30px; text-align: center; background-color: #dc2626;">
                            <h1 style="margin: 0; color: #ffffff; font-size: 28px;">Password Reset Request</h1>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding: 40px 30px;">
                            <h2 style="margin: 0 0 20px 0; color: #333333; font-size: 24px;">Hello {{.Name}}!</h2>
                            <p style="margin: 0 0 20px 0; color: #666666; font-size: 16px; line-height: 1.6;">
                                We received a request to reset your password. Click the button below to create a new password:
                            </p>
                            <table cellpadding="0" cellspacing="0" width="100%" style="margin: 30px 0;">
                                <tr>
                                    <td align="center">
                                        <a href="{{.ResetLink}}" style="display: inline-block; padding: 14px 40px; background-color: #dc2626; color: #ffffff; text-decoration: none; border-radius: 6px; font-size: 16px; font-weight: bold;">Reset Password</a>
                                    </td>
                                </tr>
                            </table>
                            <p style="margin: 0 0 20px 0; color: #666666; font-size: 16px; line-height: 1.6;">
                                This link will expire in 1 hour. If you didn't request a password reset, you can safely ignore this email.
                            </p>
                            <p style="margin: 0; color: #999999; font-size: 14px; line-height: 1.6;">
                                If the button doesn't work, copy and paste this link into your browser:<br>
                                <span style="color: #2563eb; word-break: break-all;">{{.ResetLink}}</span>
                            </p>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding: 30px; text-align: center; background-color: #f8f9fa; border-top: 1px solid #e9ecef;">
                            <p style="margin: 0; color: #999999; font-size: 14px;">
                                &copy; 2025 Lux SUV. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`

const LoginAlertTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>New Login Detected</title>
</head>
<body style="margin: 0; padding: 0; font-family: Arial, sans-serif; background-color: #f4f4f4;">
    <table cellpadding="0" cellspacing="0" width="100%" style="background-color: #f4f4f4; padding: 20px;">
        <tr>
            <td align="center">
                <table cellpadding="0" cellspacing="0" width="600" style="background-color: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
                    <tr>
                        <td style="padding: 40px 30px; text-align: center; background-color: #059669;">
                            <h1 style="margin: 0; color: #ffffff; font-size: 28px;">New Login Detected</h1>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding: 40px 30px;">
                            <h2 style="margin: 0 0 20px 0; color: #333333; font-size: 24px;">Hello {{.Name}}!</h2>
                            <p style="margin: 0 0 20px 0; color: #666666; font-size: 16px; line-height: 1.6;">
                                We detected a new login to your account. Here are the details:
                            </p>
                            <table cellpadding="10" cellspacing="0" width="100%" style="margin: 20px 0; background-color: #f8f9fa; border-radius: 6px;">
                                <tr>
                                    <td style="color: #666666; font-size: 14px;"><strong>Time:</strong></td>
                                    <td style="color: #666666; font-size: 14px;">{{.Time}}</td>
                                </tr>
                                <tr>
                                    <td style="color: #666666; font-size: 14px;"><strong>IP Address:</strong></td>
                                    <td style="color: #666666; font-size: 14px;">{{.IPAddress}}</td>
                                </tr>
                                <tr>
                                    <td style="color: #666666; font-size: 14px;"><strong>Device:</strong></td>
                                    <td style="color: #666666; font-size: 14px;">{{.UserAgent}}</td>
                                </tr>
                            </table>
                            <p style="margin: 0 0 20px 0; color: #666666; font-size: 16px; line-height: 1.6;">
                                If this was you, you can safely ignore this email. If you don't recognize this activity, please change your password immediately and contact our support team.
                            </p>
                        </td>
                    </tr>
                    <tr>
                        <td style="padding: 30px; text-align: center; background-color: #f8f9fa; border-top: 1px solid #e9ecef;">
                            <p style="margin: 0; color: #999999; font-size: 14px;">
                                &copy; 2025 Lux SUV. All rights reserved.
                            </p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`
const VerifyAccountTemplate = `
<!DOCTYPE html>
<html><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Verify your account</title></head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background:#f4f4f4;">
  <table cellpadding="0" cellspacing="0" width="100%" style="background:#f4f4f4;padding:20px;">
    <tr><td align="center">
      <table cellpadding="0" cellspacing="0" width="600" style="background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 4px rgba(0,0,0,0.1);">
        <tr>
          <td style="padding:32px 28px;text-align:center;background:#1f2937;">
            <h1 style="margin:0;color:#fff;font-size:24px;">Verify your {{.Role}} account</h1>
          </td>
        </tr>
        <tr>
          <td style="padding:32px 28px;">
            <p style="margin:0 0 16px 0;color:#374151;font-size:16px;">Hello {{.Name}},</p>
            <p style="margin:0 0 16px 0;color:#6b7280;font-size:15px;line-height:1.6;">
              Please confirm your email to continue.
            </p>
            <table cellpadding="0" cellspacing="0" width="100%" style="margin:24px 0;">
              <tr><td align="center">
                <a href="{{.VerifyLink}}" style="display:inline-block;padding:12px 28px;background:#2563eb;color:#fff;text-decoration:none;border-radius:6px;font-weight:bold;">Verify Email</a>
              </td></tr>
            </table>
            <p style="margin:0 0 10px 0;color:#9ca3af;font-size:13px;">
              This link expires in {{.ExpiresIn}}.
            </p>
            <p style="margin:0;color:#9ca3af;font-size:13px;word-break:break-all;">
              If the button doesn't work, copy/paste this URL:<br>{{.VerifyLink}}
            </p>
          </td>
        </tr>
        <tr>
          <td style="padding:18px;text-align:center;background:#f9fafb;border-top:1px solid #e5e7eb;color:#9ca3af;font-size:12px;">
            &copy; 2025 Lux SUV. All rights reserved.
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body></html>
`

const DriverVerifiedAdminAlertTemplate = `
<!DOCTYPE html>
<html><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Driver email verified</title></head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background:#f4f4f4;">
  <table cellpadding="0" cellspacing="0" width="100%" style="background:#f4f4f4;padding:20px;">
    <tr><td align="center">
      <table cellpadding="0" cellspacing="0" width="600" style="background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 4px rgba(0,0,0,0.1);">
        <tr>
          <td style="padding:24px 20px;text-align:center;background:#111827;">
            <h1 style="margin:0;color:#fff;font-size:20px;">Driver email verified – review needed</h1>
          </td>
        </tr>
        <tr>
          <td style="padding:24px 20px;color:#374151;font-size:15px;line-height:1.6;">
            <p style="margin:0 0 10px 0;">A driver has verified their email.</p>
            <ul style="margin:0 0 14px 18px;color:#4b5563;">
				<li><strong>ApplicationID:</strong> {{.AppID}}</li>
              <li><strong>UserID:</strong> {{.UserID}}</li>
              <li><strong>Email:</strong> {{.Email}}</li>
              <li><strong>VerifiedAt:</strong> {{.VerifiedAt}}</li>
            </ul>
            <p style="margin:0 0 10px 0;">Please review their application in the Admin Console.</p>
            {{if .AdminLink}}
            <p style="margin:0;"><a href="{{.AdminLink}}" style="color:#2563eb;">Open Admin Console</a></p>
            {{end}}
          </td>
        </tr>
        <tr>
          <td style="padding:16px;text-align:center;background:#f9fafb;border-top:1px solid #e5e7eb;color:#9ca3af;font-size:12px;">
            &copy; 2025 Lux SUV. All rights reserved.
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body></html>
`
const SuspiciousAdminRecoveryAlertTemplate = `
<!DOCTYPE html>
<html><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<title>Suspicious admin password recovery attempt</title></head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background:#f4f4f4;">
  <table cellpadding="0" cellspacing="0" width="100%" style="background:#f4f4f4;padding:20px;">
    <tr><td align="center">
      <table cellpadding="0" cellspacing="0" width="600" style="background:#fff;border-radius:8px;overflow:hidden;box-shadow:0 2px 4px rgba(0,0,0,0.08);">
        <tr>
          <td style="padding:24px 20px;text-align:center;background:#7c2d12;">
            <h1 style="margin:0;color:#fff;font-size:20px;">Admin Password Recovery Attempt</h1>
          </td>
        </tr>
        <tr>
          <td style="padding:24px 20px;color:#374151;font-size:15px;line-height:1.6;">
            <p style="margin:0 0 10px 0;">We detected a password reset attempt for an admin-class account.</p>
            <ul style="margin:10px 0 14px 18px;color:#4b5563;">
              <li><strong>Time (UTC):</strong> {{.AttemptTime}}</li>
              <li><strong>Target Email:</strong> {{.AdminEmail}}</li>
              <li><strong>IP Address:</strong> {{.IPAddress}}</li>
              {{if .UserAgent}}<li><strong>User Agent:</strong> {{.UserAgent}}</li>{{end}}
            </ul>
            <p style="margin:0 0 12px 0;">If this was you, you can ignore this message. If it wasn’t you, consider the following:</p>
            <ul style="margin:0 0 14px 18px;color:#4b5563;">
              <li>Rotate your password and enable MFA on the account.</li>
              <li>Review recent login activity and IP allow/block lists.</li>
            </ul>
            {{if .AdminConsoleURL}}
            <p style="margin:0;"><a href="{{.AdminConsoleURL}}" style="color:#2563eb;">Open Admin Console</a></p>
            {{end}}
          </td>
        </tr>
        <tr>
          <td style="padding:16px;text-align:center;background:#f9fafb;border-top:1px solid #e5e7eb;color:#9ca3af;font-size:12px;">
            &copy; 2025 Lux SUV. All rights reserved.
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body></html>
`
