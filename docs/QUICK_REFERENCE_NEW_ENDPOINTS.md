# Quick Reference: New WhatsApp Gateway Endpoints

## 🚀 Quick Start

Semua endpoint menggunakan prefix `/wa` dan memerlukan token authentication (kecuali admin routes).

**Base URL:** `https://api-wa.genfity.com`  
**Authentication:** Header `token: your_user_token`

---

## 📋 Endpoint Summary

| Category | Method | Endpoint | Description |
|----------|--------|----------|-------------|
| **HMAC** | POST | `/wa/session/hmac/config` | Configure HMAC key |
| **HMAC** | GET | `/wa/session/hmac/config` | Get HMAC status |
| **HMAC** | DELETE | `/wa/session/hmac/config` | Remove HMAC |
| **History** | POST | `/wa/session/history` | Configure history storage |
| **History** | GET | `/wa/chat/history` | Get chat messages |
| **Poll** | POST | `/wa/chat/send/poll` | Send poll to group |
| **Status** | POST | `/wa/status/set/text` | Set profile status |
| **User** | GET | `/wa/user/lid/:jid` | Get user LID |

---

## ⚡ Quick Examples

### 1️⃣ Enable History (500 messages per chat)
```bash
curl -X POST https://api-wa.genfity.com/wa/session/history \
  -H "token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"history": 500}'
```

### 2️⃣ Get Chat History
```bash
curl "https://api-wa.genfity.com/wa/chat/history?chat_jid=6281234567890@s.whatsapp.net&limit=50" \
  -H "token: YOUR_TOKEN"
```

### 3️⃣ Configure HMAC (Secure Webhooks)
```bash
curl -X POST https://api-wa.genfity.com/wa/session/hmac/config \
  -H "token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"hmac_key": "your_secret_key_min_32_characters_long_123456"}'
```

### 4️⃣ Send Poll
```bash
curl -X POST https://api-wa.genfity.com/wa/chat/send/poll \
  -H "token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "group": "120363313346913103@g.us",
    "header": "Meeting time?",
    "options": ["9 AM", "2 PM", "6 PM"]
  }'
```

### 5️⃣ Update Status
```bash
curl -X POST https://api-wa.genfity.com/wa/status/set/text \
  -H "token: YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"Body": "Available 24/7"}'
```

### 6️⃣ Get User LID
```bash
curl "https://api-wa.genfity.com/wa/user/lid/6281234567890@s.whatsapp.net" \
  -H "token: YOUR_TOKEN"
```

---

## 🔑 HMAC Verification (Node.js)

```javascript
const crypto = require('crypto');

function verifyWebhook(payload, signature, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  const digest = 'sha256=' + hmac.update(payload).digest('hex');
  
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(digest)
  );
}

// Usage in Express
app.post('/webhook', (req, res) => {
  const signature = req.headers['x-webhook-signature'];
  const payload = JSON.stringify(req.body);
  const secret = 'your_hmac_key_here';
  
  if (verifyWebhook(payload, signature, secret)) {
    // Signature valid - process webhook
    console.log('Valid webhook:', req.body);
    res.status(200).send('OK');
  } else {
    // Invalid signature
    res.status(401).send('Invalid signature');
  }
});
```

---

## 🔑 HMAC Verification (PHP)

```php
<?php
function verifyWebhook($payload, $signature, $secret) {
    $expectedSignature = 'sha256=' . hash_hmac('sha256', $payload, $secret);
    return hash_equals($expectedSignature, $signature);
}

// Usage
$payload = file_get_contents('php://input');
$signature = $_SERVER['HTTP_X_WEBHOOK_SIGNATURE'];
$secret = 'your_hmac_key_here';

if (verifyWebhook($payload, $signature, $secret)) {
    // Valid webhook
    $data = json_decode($payload, true);
    echo "Valid webhook\n";
} else {
    // Invalid signature
    http_response_code(401);
    echo "Invalid signature\n";
}
?>
```

---

## 🔑 HMAC Verification (Python)

```python
import hmac
import hashlib

def verify_webhook(payload, signature, secret):
    expected_signature = 'sha256=' + hmac.new(
        secret.encode(),
        payload.encode(),
        hashlib.sha256
    ).hexdigest()
    
    return hmac.compare_digest(expected_signature, signature)

# Usage in Flask
from flask import Flask, request

app = Flask(__name__)

@app.route('/webhook', methods=['POST'])
def webhook():
    payload = request.get_data(as_text=True)
    signature = request.headers.get('X-Webhook-Signature')
    secret = 'your_hmac_key_here'
    
    if verify_webhook(payload, signature, secret):
        # Valid webhook
        data = request.get_json()
        print('Valid webhook:', data)
        return 'OK', 200
    else:
        # Invalid signature
        return 'Invalid signature', 401
```

---

## 📊 Response Formats

### ✅ Success Response
```json
{
  "status": "success",
  "message": "Operation completed successfully",
  "data": { ... }
}
```

### ❌ Error Responses

**401 Unauthorized**
```json
{
  "status": 401,
  "message": "Token required"
}
```

**403 Forbidden**
```json
{
  "status": 403,
  "message": "No active subscription found"
}
```

**400 Bad Request**
```json
{
  "status": 400,
  "message": "Invalid request parameters"
}
```

---

## 💡 Pro Tips

### History Management
```bash
# Enable with 1000 messages limit
curl -X POST /wa/session/history -H "token: TOKEN" -d '{"history": 1000}'

# Disable history
curl -X POST /wa/session/history -H "token: TOKEN" -d '{"history": 0}'

# Get all chats index
curl "/wa/chat/history?chat_jid=index" -H "token: TOKEN"
```

### Poll Best Practices
- ✅ Use clear, concise questions
- ✅ 2-5 options untuk best results
- ✅ Test poll di test group dulu
- ❌ Jangan send ke personal chat (akan error)

### HMAC Security
- ✅ Generate key dengan `openssl rand -hex 32`
- ✅ Store key di environment variables
- ✅ Rotate keys setiap 90 days
- ❌ Never hardcode keys dalam source code

---

## 🎯 Common Use Cases

### Use Case 1: Customer Service with History
```bash
# 1. Enable history untuk customer service
curl -X POST /wa/session/history \
  -H "token: TOKEN" -d '{"history": 1000}'

# 2. Agent bisa lihat history chat customer
curl "/wa/chat/history?chat_jid=CUSTOMER_JID&limit=100" \
  -H "token: TOKEN"
```

### Use Case 2: Secure Webhook Integration
```bash
# 1. Configure HMAC
curl -X POST /wa/session/hmac/config \
  -H "token: TOKEN" \
  -d '{"hmac_key": "SECURE_RANDOM_KEY_32_CHARS"}'

# 2. Implement verification di webhook endpoint
# (See code examples above)
```

### Use Case 3: Group Engagement
```bash
# 1. Set engaging status
curl -X POST /wa/status/set/text \
  -H "token: TOKEN" \
  -d '{"Body": "Online - Ready to help! 🚀"}'

# 2. Send interactive poll
curl -X POST /wa/chat/send/poll \
  -H "token: TOKEN" \
  -d '{
    "group": "GROUP_JID",
    "header": "Rate our service!",
    "options": ["⭐⭐⭐⭐⭐ Excellent", "⭐⭐⭐⭐ Good", "⭐⭐⭐ Average", "⭐⭐ Poor"]
  }'
```

---

## ⚠️ Important Limits

| Feature | Limit | Notes |
|---------|-------|-------|
| History Storage | 10,000 per chat | Recommended: 500-1000 |
| HMAC Key Length | Min 32 chars | Use secure random string |
| Poll Options | 2-12 options | Single choice only |
| Poll Destination | Groups only | Cannot send to personal |
| Status Text Length | 139 chars | Similar to Twitter |

---

## 🔍 Debugging

### Check Token Status
```bash
curl -X GET https://api-wa.genfity.com/wa/session/status \
  -H "token: YOUR_TOKEN"
```

### Check HMAC Configuration
```bash
curl -X GET https://api-wa.genfity.com/wa/session/hmac/config \
  -H "token: YOUR_TOKEN"
```

### Test History Setup
```bash
# Enable history
curl -X POST /wa/session/history -H "token: TOKEN" -d '{"history": 10}'

# Send test message
curl -X POST /wa/chat/send/text -H "token: TOKEN" \
  -d '{"Phone": "6281234567890", "Body": "Test message"}'

# Retrieve history
curl "/wa/chat/history?chat_jid=6281234567890@s.whatsapp.net&limit=10" \
  -H "token: TOKEN"
```

---

## 📚 More Documentation

- **Complete API Details:** `NEW_ENDPOINTS_DOCUMENTATION.md`
- **Gateway Overview:** `GATEWAY_DOCUMENTATION.md`
- **All Changes:** `CHANGELOG_GATEWAY_UPDATE.md`
- **Chat API:** `API_CHAT_DOCUMENTATION.md`
- **Webhook Info:** `WEBHOOK_DOCUMENTATION.md`

---

## 🆘 Troubleshooting

### Problem: History returns empty
**Solution:** 
1. Ensure history is enabled: `POST /wa/session/history` with `{"history": 500}`
2. Send some messages after enabling
3. Wait a few seconds for messages to be stored

### Problem: HMAC verification fails
**Solution:**
1. Verify key length >= 32 characters
2. Check key matches exactly (no extra spaces)
3. Use same key in both configuration and verification

### Problem: Poll not sending
**Solution:**
1. Verify sending to group JID (ends with `@g.us`)
2. Check you have at least 2 options
3. Ensure options array has max 12 items

### Problem: 403 Forbidden error
**Solution:**
1. Check subscription is active
2. Verify token is correct
3. Ensure subscription not expired

---

## 🎉 Ready to Use!

```bash
# Quick test - Enable all features
TOKEN="your_token_here"

# 1. Enable history
curl -X POST https://api-wa.genfity.com/wa/session/history \
  -H "token: $TOKEN" -d '{"history": 500}'

# 2. Configure HMAC
curl -X POST https://api-wa.genfity.com/wa/session/hmac/config \
  -H "token: $TOKEN" \
  -d '{"hmac_key": "secure_random_key_minimum_32_characters_long_12345"}'

# 3. Update status
curl -X POST https://api-wa.genfity.com/wa/status/set/text \
  -H "token: $TOKEN" \
  -d '{"Body": "Powered by Genfity WA API 🚀"}'

echo "✅ All features enabled!"
```

---

**Last Updated:** November 6, 2025  
**API Version:** 2.0.0
