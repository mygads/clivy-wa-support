# API Documentation - Chat Room System

## Overview
This API provides WhatsApp-like chat room functionality with message status tracking similar to WhatsApp Web. Each user can view their chat rooms, messages, and track message delivery status (sent ✓, delivered ✓✓, read ✓✓ blue).

## Authentication

### Admin & User Endpoints
Both admin and user endpoints require Bearer token authentication:
```
Authorization: Bearer your_admin_token_here
```
Set `ADMIN_TOKEN` in your .env file.

### Public Endpoints
These endpoints do not require authentication:
- `/health`
- `/sessions/sync`
- `/webhook/wa` (GET and POST)

## Admin Endpoints

All admin endpoints require Bearer token authentication.

### GET /admin/users
Retrieve all users and their settings.

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 10, max: 100)

**Response:**
```json
{
  "users": [
    {
      "user_token": "user123",
      "chat_log_enabled": true,
      "auto_read_enabled": false,
      "display_name": "John Doe",
      "phone_number": "+1234567890",
      "is_active": true,
      "session_state": "connected",
      "connected": true,
      "logged_in": true,
      "jid": "1234567890@s.whatsapp.net",
      "message_count": 150,
      "chat_room_count": 5,
      "last_activity": "2025-08-15T10:30:00Z"
    }
  ],
  "total": 25,
  "page": 1,
  "limit": 10,
  "pages": 3
}
```

### GET /admin/users/:user_token
Retrieve specific user details.

**Response:**
```json
{
  "user_settings": {
    "id": 1,
    "user_token": "user123",
    "chat_log_enabled": true,
    "auto_read_enabled": false,
    "webhook_url": "",
    "display_name": "John Doe",
    "phone_number": "+1234567890",
    "is_active": true,
    "created_at": "2025-08-15T09:00:00Z",
    "updated_at": "2025-08-15T10:30:00Z"
  },
  "session": {
    "id": 1,
    "user_token": "user123",
    "session_state": "connected",
    "connected": true,
    "logged_in": true,
    "jid": "1234567890@s.whatsapp.net",
    "last_activity_at": "2025-08-15T10:30:00Z"
  },
  "stats": {
    "message_count": 150,
    "chat_room_count": 5,
    "unread_chat_count": 2
  }
}
```

### PUT /admin/users/:user_token/update
Update user settings.

**Request Body:**
```json
{
  "chat_log_enabled": true,
  "auto_read_enabled": false,
  "webhook_url": "https://your-webhook.com/wa",
  "display_name": "John Doe",
  "phone_number": "+1234567890",
  "is_active": true
}
```

**Response:**
```json
{
  "status": "success",
  "message": "User settings updated successfully",
  "user_settings": {
    "id": 1,
    "user_token": "user123",
    "chat_log_enabled": true,
    "auto_read_enabled": false,
    "webhook_url": "https://your-webhook.com/wa",
    "display_name": "John Doe",
    "phone_number": "+1234567890",
    "is_active": true,
    "created_at": "2025-08-15T09:00:00Z",
    "updated_at": "2025-08-15T10:30:00Z"
  }
}
```

### GET /admin/sessions
Retrieve all WhatsApp sessions.

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 10, max: 100)
- `session_state` (optional): Filter by session state

**Response:**
```json
{
  "sessions": [
    {
      "id": 1,
      "user_token": "user123",
      "session_name": "John's Phone",
      "session_id": "session_123",
      "jid": "1234567890@s.whatsapp.net",
      "session_state": "connected",
      "connected": true,
      "logged_in": true,
      "qr_code": "",
      "connected_at": "2025-08-15T09:00:00Z",
      "last_activity_at": "2025-08-15T10:30:00Z"
    }
  ],
  "total": 10,
  "page": 1,
  "limit": 10,
  "pages": 1
}
```

### GET /admin/sessions/:user_token
Retrieve session for specific user.

**Response:**
```json
{
  "session": {
    "id": 1,
    "user_token": "user123",
    "session_name": "John's Phone",
    "session_id": "session_123",
    "jid": "1234567890@s.whatsapp.net",
    "session_state": "connected",
    "connected": true,
    "logged_in": true,
    "qr_code": "",
    "connected_at": "2025-08-15T09:00:00Z",
    "last_activity_at": "2025-08-15T10:30:00Z"
  }
}
```

### GET /admin/event
Retrieve all events across all users.

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 10, max: 100)
- `event_type` (optional): Filter by event type
- `source` (optional): Filter by source (default: "wa")
- `user_token` (optional): Filter by specific user

**Response:**
```json
{
  "events": [
    {
      "id": 1,
      "event_type": "Message",
      "source": "wa",
      "user_token": "user123",
      "event_data": {...},
      "raw_data": "...",
      "processed": true,
      "received_at": "2025-08-15T10:30:00Z",
      "processed_at": "2025-08-15T10:30:05Z"
    }
  ],
  "total": 500,
  "page": 1,
  "limit": 10,
  "pages": 50
}
```

## User Endpoints

All user endpoints require Bearer token authentication and are filtered by `user_token`.

### GET /user/chat/:user_token
Retrieve chat rooms for a specific user.

**Headers:**
```
Authorization: Bearer your_admin_token_here
```

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20, max: 100)
- `search` (optional): Search by contact name, JID, or group name

**Response:**
```json
{
  "chat_rooms": [
    {
      "id": 1,
      "chat_id": "user123_1234567891@s.whatsapp.net",
      "user_token": "user123",
      "contact_jid": "1234567891@s.whatsapp.net",
      "contact_name": "Alice Smith",
      "chat_type": "individual",
      "is_group": false,
      "group_name": "",
      "last_message": "Hello, how are you?",
      "last_sender": "contact",
      "last_activity": "2025-08-15T10:30:00Z",
      "unread_count": 2,
      "created_at": "2025-08-15T09:00:00Z",
      "updated_at": "2025-08-15T10:30:00Z"
    }
  ],
  "total": 5,
  "page": 1,
  "limit": 20,
  "pages": 1
}
```

### GET /user/message/:user_token/:chat_id
Retrieve messages for a specific chat.

**Headers:**
```
Authorization: Bearer your_admin_token_here
```

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 50, max: 100)
- `message_type` (optional): Filter by message type

**Response:**
```json
{
  "messages": [
    {
      "id": 1,
      "message_id": "msg_123",
      "chat_id": "user123_1234567891@s.whatsapp.net",
      "user_token": "user123",
      "sender_jid": "1234567891@s.whatsapp.net",
      "sender_type": "contact",
      "message_type": "text",
      "content": "Hello, how are you?",
      "caption": "",
      "media_data": null,
      "quoted_message_id": "",
      "status": "delivered",
      "message_timestamp": "2025-08-15T10:30:00Z",
      "delivered_at": "2025-08-15T10:30:02Z",
      "read_at": null,
      "created_at": "2025-08-15T10:30:00Z",
      "updated_at": "2025-08-15T10:30:02Z"
    }
  ],
  "chat_room": {
    "id": 1,
    "chat_id": "user123_1234567891@s.whatsapp.net",
    "user_token": "user123",
    "contact_jid": "1234567891@s.whatsapp.net",
    "contact_name": "Alice Smith",
    "chat_type": "individual",
    "is_group": false,
    "last_message": "Hello, how are you?",
    "last_sender": "contact",
    "last_activity": "2025-08-15T10:30:00Z",
    "unread_count": 0
  },
  "total": 25,
  "page": 1,
  "limit": 50,
  "pages": 1
}
```

### GET /user/event/:user_token
Retrieve events for a specific user.

**Headers:**
```
Authorization: Bearer your_admin_token_here
```

**Query Parameters:**
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20, max: 100)
- `event_type` (optional): Filter by event type
- `source` (optional): Filter by source (default: "wa")

**Response:**
```json
{
  "events": [
    {
      "id": 1,
      "event_type": "Message",
      "source": "wa",
      "user_token": "user123",
      "event_data": {...},
      "raw_data": "...",
      "processed": true,
      "received_at": "2025-08-15T10:30:00Z",
      "processed_at": "2025-08-15T10:30:05Z"
    }
  ],
  "total": 100,
  "page": 1,
  "limit": 20,
  "pages": 5
}
```

## Public Endpoints (No Authentication Required)

### GET /health
Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "time": "2025-08-15T10:30:00Z",
  "service": "genfity-wa-support"
}
```

### GET /sessions/sync
Sync session status with WhatsApp server.

**Response:**
```json
{
  "status": "success",
  "message": "Session status synced successfully",
  "stats": {
    "total_sessions": 5,
    "created_sessions": 1,
    "updated_sessions": 4,
    "last_sync_at": "2025-08-15T10:30:00Z"
  }
}
```

### GET /webhook/wa
WhatsApp webhook verification (for platform setup).

### POST /webhook/wa
WhatsApp webhook handler (receives events from WhatsApp server).

## Message Status Tracking

Messages have three status levels similar to WhatsApp:

1. **sent** (✓): Message sent from user
2. **delivered** (✓✓): Message delivered to recipient
3. **read** (✓✓ blue): Message read by recipient

For incoming messages, the status is automatically set to "delivered" since they've reached our system.

## User Settings

### Chat Log Feature
- `chat_log_enabled`: Controls whether to store chat messages in the database
- Default: `false` (disabled)
- When disabled, webhook events are still stored, but messages are not saved to chat rooms
- When sync session is performed, this setting is reset to `false` for new users

### Auto Read Feature
- `auto_read_enabled`: Automatically mark messages as read when user views them
- Default: `false`
- When enabled, viewing chat messages will mark them as read and reset unread count

## Setup Instructions

1. Set up your environment variables in `.env` file
2. Configure `ADMIN_TOKEN` for admin authentication
3. Configure WhatsApp server settings if using session sync
4. Run database migrations (handled automatically)
5. Start the server with `./app.exe`

## Error Responses

All endpoints return consistent error responses:

```json
{
  "error": "Error description"
}
```

Common HTTP status codes:
- `400`: Bad Request (invalid parameters)
- `401`: Unauthorized (missing/invalid admin token)
- `403`: Forbidden (chat log disabled for user)
- `404`: Not Found (user/chat/message not found)
- `500`: Internal Server Error
