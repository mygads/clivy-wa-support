# New WhatsApp API Endpoints Documentation

## Overview
This document describes the new endpoints added to the WhatsApp Gateway API.

---

## 1. HMAC Configuration Endpoints

HMAC (Hash-based Message Authentication Code) is used to sign webhook payloads for secure verification.

### Configure HMAC Key
**Endpoint:** `POST /wa/session/hmac/config`

**Description:** Configure HMAC key for webhook signing. The key must be at least 32 characters long.

**Headers:**
```
token: your_user_token
Content-Type: application/json
```

**Request Body:**
```json
{
  "hmac_key": "your_hmac_key_minimum_32_characters_long_here"
}
```

**Response Success (200):**
```json
{
  "status": "success",
  "message": "HMAC key configured successfully"
}
```

**Response Error (400):**
```json
{
  "status": "error",
  "message": "HMAC key must be at least 32 characters"
}
```

---

### Get HMAC Configuration
**Endpoint:** `GET /wa/session/hmac/config`

**Description:** Get HMAC configuration status.

**Headers:**
```
token: your_user_token
```

**Response Success (200):**
```json
{
  "status": "success",
  "hmac_enabled": true,
  "hmac_key": "your_hmac_key...***" // Partially masked
}
```

---

### Delete HMAC Configuration
**Endpoint:** `DELETE /wa/session/hmac/config`

**Description:** Delete HMAC configuration and disable webhook signing.

**Headers:**
```
token: your_user_token
```

**Response Success (200):**
```json
{
  "status": "success",
  "message": "HMAC configuration deleted successfully"
}
```

---

## 2. Message History Endpoints

Message history allows you to store and retrieve chat messages.

### Configure Message History
**Endpoint:** `POST /wa/session/history`

**Description:** Configure message history storage. Set history to 0 to disable, or any positive number to enable with that limit.

**Headers:**
```
token: your_user_token
Content-Type: application/json
```

**Request Body:**
```json
{
  "history": 500
}
```

**Parameters:**
- `history` (integer): Number of messages to store per chat. Use 0 to disable, or positive number (e.g., 500) to enable.

**Response Success (200):**
```json
{
  "status": "success",
  "message": "Message history configured to store 500 messages per chat"
}
```

**Example - Disable History:**
```json
{
  "history": 0
}
```

**Response Success (200):**
```json
{
  "status": "success",
  "message": "Message history disabled"
}
```

---

### Get Message History
**Endpoint:** `GET /wa/chat/history`

**Description:** Retrieve message history for a specific chat. Requires history to be enabled via POST `/session/history`.

**Headers:**
```
token: your_user_token
```

**Query Parameters:**
- `chat_jid` (string, required): JID of the chat to retrieve history from. Use 'index' to get all chats mapping.
- `limit` (integer, optional): Number of messages to retrieve. Default: 50.

**Example Request:**
```
GET /wa/chat/history?chat_jid=628123456789@s.whatsapp.net&limit=50
```

**Response Success (200):**
```json
{
  "status": "success",
  "chat_jid": "628123456789@s.whatsapp.net",
  "messages": [
    {
      "id": "AABBCC11223344",
      "from": "628123456789@s.whatsapp.net",
      "timestamp": 1699876543,
      "message_type": "text",
      "text": "Hello, how are you?",
      "quoted": null
    },
    {
      "id": "DDEEFF55667788",
      "from": "me",
      "timestamp": 1699876600,
      "message_type": "text",
      "text": "I'm fine, thanks!",
      "quoted": null
    }
  ],
  "total": 2
}
```

**Example - Get All Chats:**
```
GET /wa/chat/history?chat_jid=index
```

**Response Success (200):**
```json
{
  "status": "success",
  "chats": [
    {
      "jid": "628123456789@s.whatsapp.net",
      "name": "John Doe",
      "message_count": 150
    },
    {
      "jid": "120363313346913103@g.us",
      "name": "My Group",
      "message_count": 500
    }
  ]
}
```

---

## 3. Status Endpoint

### Set Status Text
**Endpoint:** `POST /wa/status/set/text`

**Description:** Set WhatsApp profile status message.

**Headers:**
```
token: your_user_token
Content-Type: application/json
```

**Request Body:**
```json
{
  "Body": "Available - Powered by Genfity WA"
}
```

**Response Success (200):**
```json
{
  "status": "success",
  "message": "Status updated successfully"
}
```

---

## 4. User LID Endpoint

### Get User LID
**Endpoint:** `GET /wa/user/lid/:jid`

**Description:** Get User Linked ID (LID) for a specific JID.

**Headers:**
```
token: your_user_token
```

**Path Parameters:**
- `jid` (string): The WhatsApp JID (e.g., `628123456789@s.whatsapp.net`)

**Example Request:**
```
GET /wa/user/lid/628123456789@s.whatsapp.net
```

**Response Success (200):**
```json
{
  "status": "success",
  "jid": "628123456789@s.whatsapp.net",
  "lid": "628123456789:10@lid"
}
```

---

## 5. Send Poll Endpoint

### Send Poll
**Endpoint:** `POST /wa/chat/send/poll`

**Description:** Send a poll to a group. Minimum 2 options required. Maximum 1 selection allowed.

**Headers:**
```
token: your_user_token
Content-Type: application/json
```

**Request Body:**
```json
{
  "group": "120363313346913103@g.us",
  "header": "What is your favorite color?",
  "options": ["Red", "Blue", "Green", "Yellow"],
  "Id": ""
}
```

**Parameters:**
- `group` (string, required): Group JID where poll will be sent
- `header` (string, required): Poll question/header
- `options` (array, required): Array of poll options (minimum 2, maximum 12)
- `Id` (string, optional): Message ID for tracking

**Response Success (200):**
```json
{
  "status": "success",
  "message_id": "AABBCC11223344",
  "timestamp": 1699876543
}
```

**Response Error (400):**
```json
{
  "status": "error",
  "message": "Poll must have at least 2 options"
}
```

---

## Complete Usage Examples

### Example 1: Enable History and Retrieve Messages

**Step 1: Enable History**
```bash
curl -X POST https://api-wa.genfity.com/wa/session/history \
  -H "token: your_token_here" \
  -H "Content-Type: application/json" \
  -d '{
    "history": 500
  }'
```

**Step 2: Get Chat History**
```bash
curl -X GET "https://api-wa.genfity.com/wa/chat/history?chat_jid=628123456789@s.whatsapp.net&limit=50" \
  -H "token: your_token_here"
```

---

### Example 2: Configure HMAC for Secure Webhooks

**Step 1: Set HMAC Key**
```bash
curl -X POST https://api-wa.genfity.com/wa/session/hmac/config \
  -H "token: your_token_here" \
  -H "Content-Type: application/json" \
  -d '{
    "hmac_key": "my_super_secret_key_at_least_32_chars_long_12345"
  }'
```

**Step 2: Verify HMAC Configuration**
```bash
curl -X GET https://api-wa.genfity.com/wa/session/hmac/config \
  -H "token: your_token_here"
```

---

### Example 3: Send Poll to Group

```bash
curl -X POST https://api-wa.genfity.com/wa/chat/send/poll \
  -H "token: your_token_here" \
  -H "Content-Type: application/json" \
  -d '{
    "group": "120363313346913103@g.us",
    "header": "What time works best for our meeting?",
    "options": ["9:00 AM", "2:00 PM", "4:00 PM", "6:00 PM"],
    "Id": ""
  }'
```

---

### Example 4: Update Status Message

```bash
curl -X POST https://api-wa.genfity.com/wa/status/set/text \
  -H "token: your_token_here" \
  -H "Content-Type: application/json" \
  -d '{
    "Body": "Available 24/7 - Automated by Genfity"
  }'
```

---

### Example 5: Get User LID

```bash
curl -X GET https://api-wa.genfity.com/wa/user/lid/628123456789@s.whatsapp.net \
  -H "token: your_token_here"
```

---

## Error Handling

All endpoints follow the standard error response format:

### 401 Unauthorized
```json
{
  "status": 401,
  "message": "Token required"
}
```

### 403 Forbidden
```json
{
  "status": 403,
  "message": "No active subscription found"
}
```

### 400 Bad Request
```json
{
  "status": 400,
  "message": "Invalid request parameters"
}
```

### 502 Bad Gateway
```json
{
  "status": 502,
  "message": "Failed to reach WhatsApp server"
}
```

---

## Security Considerations

### HMAC Webhook Signing
When HMAC is configured, all webhook payloads will include a signature header:

```
X-Webhook-Signature: sha256=abc123def456...
```

To verify the signature on your webhook endpoint:

```javascript
const crypto = require('crypto');

function verifyWebhookSignature(payload, signature, secret) {
  const hmac = crypto.createHmac('sha256', secret);
  const digest = 'sha256=' + hmac.update(payload).digest('hex');
  return crypto.timingSafeEqual(
    Buffer.from(signature),
    Buffer.from(digest)
  );
}
```

---

## Notes

1. **Message History Storage**:
   - History is stored in memory and will be lost on server restart
   - Configure an appropriate limit based on your memory availability
   - Recommended: 500-1000 messages per chat for production

2. **Poll Limitations**:
   - Can only be sent to groups, not individual chats
   - Minimum 2 options, maximum 12 options
   - Single-choice polls only (no multi-select)

3. **HMAC Key Requirements**:
   - Must be at least 32 characters long
   - Use a cryptographically secure random string
   - Store securely and never expose in client code

4. **LID (Linked ID)**:
   - Used for advanced WhatsApp features
   - Required for some multi-device operations
   - Format: `{phone}:{device_id}@lid`

---

## Migration Guide

If you're using the previous version of the API, here's what you need to update:

### Before (Old API)
```bash
# No history support
# No HMAC support
# No poll support
```

### After (New API)
```bash
# Enable history first
curl -X POST /wa/session/history \
  -H "token: your_token" \
  -d '{"history": 500}'

# Then retrieve history
curl -X GET "/wa/chat/history?chat_jid=...&limit=50" \
  -H "token: your_token"

# Configure HMAC for security
curl -X POST /wa/session/hmac/config \
  -H "token: your_token" \
  -d '{"hmac_key": "your_secure_key_here"}'
```

---

## Support

For issues or questions about these endpoints:
- Check the main GATEWAY_DOCUMENTATION.md for authentication details
- Verify your subscription is active
- Ensure your token has the required permissions
- Check the WA server logs for detailed error messages
