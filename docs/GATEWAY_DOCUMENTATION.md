# WhatsApp Gateway Documentation

## Overview

This gateway acts as a proxy between clients and the WhatsApp server, providing subscription validation and message tracking functionality.

## Architecture

```
Client Request → Gateway → WhatsApp Server → Gateway → Client Response
               ↓
        Subscription Validation
               ↓
        Message Stats Tracking
```

## Database Structure

The gateway uses 4 core database tables:

### 1. whatsappsession
Maps tokens to users and tracks WhatsApp connection status.

```sql
CREATE TABLE whatsappsession (
    id VARCHAR(30) PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    token VARCHAR UNIQUE NOT NULL,
    status VARCHAR DEFAULT 'disconnected',
    phone VARCHAR,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### 2. whatsappapipackage
Defines package configurations and session limits.

```sql
CREATE TABLE whatsappapipackage (
    id VARCHAR(30) PRIMARY KEY,
    name VARCHAR NOT NULL,
    description TEXT,
    max_session INTEGER DEFAULT 1,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### 3. serviceswhatsappcustomers
Tracks user subscriptions and expiration dates.

```sql
CREATE TABLE serviceswhatsappcustomers (
    id VARCHAR(30) PRIMARY KEY,
    customer_id VARCHAR NOT NULL,
    package_id VARCHAR NOT NULL,
    status BOOLEAN DEFAULT true,
    expired_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP
);
```

### 4. whatsappmessagestats
Records successful message sends for analytics.

```sql
CREATE TABLE whatsappmessagestats (
    id VARCHAR(30) PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    session_token VARCHAR NOT NULL,
    message_type VARCHAR,
    phone VARCHAR,
    message_id VARCHAR,
    status VARCHAR DEFAULT 'sent',
    sent_at TIMESTAMP,
    created_at TIMESTAMP
);
```

## Gateway Logic

### Admin Routes (/admin/*)
- **Bypass all validation** - direct proxy to WA server
- No subscription checks
- No session limits
- No message tracking

### User Routes (All other endpoints)
1. **Token Validation**
   - Extract token from `token` header or `Authorization` header
   - Look up token in `whatsappsession` table
   - Get associated `user_id`

2. **Subscription Validation**
   - Check `serviceswhatsappcustomers` for active subscription
   - Auto-expire subscriptions if `expired_at` < current time
   - Update `status = false` for expired subscriptions

3. **Session Limit Validation** (for `/session/connect` only)
   - Count current active sessions for user
   - Check against `max_session` from user's package
   - Reject if limit exceeded

4. **Message Tracking** (for successful message sends)
   - Track only successful responses (status 200-299)
   - Record in `whatsappmessagestats` table
   - Extract message type from endpoint path
   - Extract phone number from request body

## Supported Endpoints

### Admin Endpoints (No validation)
- `POST /admin/users` - Add user
- `GET /admin/users` - List users  
- `DELETE /admin/users/:id` - Delete user

### Session Endpoints
- `POST /session/connect` - Connect to WhatsApp (checks session limits)
- `POST /session/disconnect` - Disconnect
- `POST /session/logout` - Logout
- `GET /session/status` - Get status
- `GET /session/qr` - Get QR code
- `POST /session/pairphone` - Pair by phone
- `POST /session/proxy` - Set proxy
- `POST /session/s3/config` - Configure S3
- `GET /session/s3/config` - Get S3 config
- `POST /session/s3/test` - Test S3
- `DELETE /session/s3/config` - Delete S3 config
- `POST /session/hmac/config` - Configure HMAC key for webhook signing
- `GET /session/hmac/config` - Get HMAC configuration status
- `DELETE /session/hmac/config` - Delete HMAC configuration
- `POST /session/history` - Configure message history storage (0 to disable, positive number to enable)

### Webhook Endpoints
- `GET /webhook` - Get webhook
- `POST /webhook` - Set webhook
- `DELETE /webhook` - Delete webhook
- `PUT /webhook/update` - Update webhook
- `GET /webhook/events` - Get webhook events

### Chat Endpoints (Message tracking enabled)
- `POST /chat/send/text` - Send text message
- `POST /chat/send/image` - Send image
- `POST /chat/send/audio` - Send audio
- `POST /chat/send/document` - Send document
- `POST /chat/send/video` - Send video
- `POST /chat/send/sticker` - Send sticker
- `POST /chat/send/location` - Send location
- `POST /chat/send/contact` - Send contact
- `POST /chat/send/template` - Send template
- `POST /chat/send/edit` - Edit message
- `POST /chat/send/poll` - Send poll to group
- `POST /chat/markread` - Mark as read
- `POST /chat/react` - React to message
- `POST /chat/presence` - Set chat presence
- `POST /chat/delete` - Delete message
- `GET /chat/history` - Get message history for a chat (requires history to be enabled)

### User Endpoints
- `POST /user/check` - Check users
- `POST /user/info` - Get user info
- `POST /user/presence` - Set global presence
- `POST /user/avatar` - Get user avatar
- `GET /user/contacts` - Get contacts
- `GET /user/lid/:jid` - Get User Linked ID (LID) for a specific JID

### Status Endpoints
- `POST /status/set/text` - Set WhatsApp profile status message

### Group Endpoints
- `POST /group/create` - Create group
- `POST /group/locked` - Set group locked
- `POST /group/ephemeral` - Set disappearing timer
- `POST /group/photo/remove` - Remove group photo
- `GET /group/list` - List groups
- `GET /group/info` - Get group info
- `GET /group/invitelink` - Get invite link
- `POST /group/name` - Change group name
- `POST /group/photo` - Change group photo
- `POST /group/topic` - Set group topic
- `POST /group/announce` - Set group announce
- `POST /group/join` - Join group
- `POST /group/inviteinfo` - Get invite info
- `POST /group/updateparticipants` - Update participants

### Newsletter Endpoints
- `GET /newsletter/list` - List newsletters

## Configuration

### Environment Variables

```bash
# WhatsApp Server URL
WA_SERVER_URL=http://your-whatsapp-server:8080

# Transactional Database (for subscription data)
TRANSACTIONAL_DB_HOST=localhost
TRANSACTIONAL_DB_PORT=5432
TRANSACTIONAL_DB_USER=postgres
TRANSACTIONAL_DB_PASSWORD=password
TRANSACTIONAL_DB_NAME=gateway_db
TRANSACTIONAL_DB_SSLMODE=disable

# Primary Database (for webhook events)
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=webhook_db
DB_SSLMODE=disable

# Server Port
PORT=8070
```

## Request Flow Examples

### 1. Admin Request (Bypassed)
```
Client → POST /admin/users
Gateway → Direct proxy to WA server
WA Server → Response
Gateway → Return response to client
```

### 2. User Message Send (Validated)
```
Client → POST /chat/send/text (with token header)
Gateway → Validate token in whatsappsession
Gateway → Check subscription in serviceswhatsappcustomers
Gateway → Proxy to WA server
WA Server → 200 OK response
Gateway → Track message in whatsappmessagestats
Gateway → Return response to client
```

### 3. Session Connect (Session limit check)
```
Client → POST /session/connect (with token header)
Gateway → Validate token and subscription
Gateway → Count current active sessions
Gateway → Check against package max_session limit
Gateway → Proxy to WA server (if within limits)
```

## Error Responses

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

```json
{
  "status": 403,
  "message": "Subscription expired on 2024-01-15"
}
```

```json
{
  "status": 403,
  "message": "Session limit exceeded. Maximum allowed: 3, current: 3"
}
```

### 502 Bad Gateway
```json
{
  "status": 502,
  "message": "Failed to reach WhatsApp server"
}
```

## Features

✅ **Simple Proxy** - Forwards all requests without modification  
✅ **Admin Bypass** - Admin routes skip all validation  
✅ **Token Validation** - Validates tokens via whatsappsession table  
✅ **Subscription Management** - Auto-expires based on date  
✅ **Session Limits** - Enforces max sessions per package  
✅ **Message Tracking** - Records successful message sends  
✅ **Multi-endpoint Support** - Handles all WhatsApp API endpoints  
✅ **Error Handling** - Proper error responses and logging  

## Usage

1. Start the gateway server
2. Configure WA_SERVER_URL to point to your WhatsApp server
3. Set up database tables with proper data
4. Send requests to gateway instead of direct WA server
5. Use `/admin/*` routes for administrative access without validation
6. Use other routes with proper token authentication
