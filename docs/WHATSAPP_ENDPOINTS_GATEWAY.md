# WhatsApp Server Endpoints yang Harus Menggunakan Gateway

## Overview

Berikut adalah daftar lengkap endpoint WhatsApp API yang harus diarahkan melalui gateway untuk verifikasi langganan dan rate limiting. Semua endpoint ini akan diproxy ke server WhatsApp setelah melewati validasi.

## Base URL Gateway
```
http://localhost:8070/api/wa/
```

## 📬 Message Endpoints (Rate Limited)

Endpoint-endpoint ini akan dikenakan rate limiting dan tracking statistik:

### Send Messages
```bash
POST /api/wa/send-message          # Kirim pesan teks
POST /api/wa/send-media            # Kirim media (gambar, video, audio, document)
POST /api/wa/send-image            # Kirim gambar khusus
POST /api/wa/send-video            # Kirim video khusus
POST /api/wa/send-audio            # Kirim audio khusus
POST /api/wa/send-document         # Kirim dokumen
POST /api/wa/send-sticker          # Kirim sticker
POST /api/wa/send-location         # Kirim lokasi
POST /api/wa/send-contact          # Kirim kontak
POST /api/wa/send-poll             # Kirim poll/voting
POST /api/wa/send-list             # Kirim list message
POST /api/wa/send-button           # Kirim button message
POST /api/wa/send-template         # Kirim template message
POST /api/wa/send-reaction         # Kirim reaksi emoji
POST /api/wa/forward-message       # Forward pesan
```

### Bulk/Broadcast Messages
```bash
POST /api/wa/broadcast             # Broadcast ke multiple contacts
POST /api/wa/send-bulk             # Bulk send dengan template
```

## 🔧 Session Management Endpoints

### Session Operations
```bash
GET    /api/wa/sessions                    # List all sessions
POST   /api/wa/sessions                    # Create new session
GET    /api/wa/sessions/{session_id}       # Get session details
PUT    /api/wa/sessions/{session_id}       # Update session settings
DELETE /api/wa/sessions/{session_id}       # Delete session
```

### Session Control
```bash
POST   /api/wa/sessions/{session_id}/start    # Start session
POST   /api/wa/sessions/{session_id}/stop     # Stop session
POST   /api/wa/sessions/{session_id}/restart  # Restart session
POST   /api/wa/sessions/{session_id}/logout   # Logout session
GET    /api/wa/sessions/{session_id}/status   # Get session status
```

### QR Code & Authentication
```bash
GET    /api/wa/sessions/{session_id}/qr       # Get QR code for pairing
POST   /api/wa/sessions/{session_id}/pair     # Pair with phone number
GET    /api/wa/sessions/{session_id}/auth     # Check auth status
```

## 👥 Contact & Chat Management

### Contacts
```bash
GET    /api/wa/sessions/{session_id}/contacts           # Get all contacts
GET    /api/wa/sessions/{session_id}/contacts/{jid}     # Get contact details
POST   /api/wa/sessions/{session_id}/contacts           # Add contact
PUT    /api/wa/sessions/{session_id}/contacts/{jid}     # Update contact
DELETE /api/wa/sessions/{session_id}/contacts/{jid}     # Delete contact
GET    /api/wa/sessions/{session_id}/contacts/blocked   # Get blocked contacts
POST   /api/wa/sessions/{session_id}/contacts/{jid}/block   # Block contact
POST   /api/wa/sessions/{session_id}/contacts/{jid}/unblock # Unblock contact
```

### Chats
```bash
GET    /api/wa/sessions/{session_id}/chats              # Get all chats
GET    /api/wa/sessions/{session_id}/chats/{jid}        # Get specific chat
POST   /api/wa/sessions/{session_id}/chats/{jid}/clear  # Clear chat history
POST   /api/wa/sessions/{session_id}/chats/{jid}/delete # Delete chat
GET    /api/wa/sessions/{session_id}/chats/{jid}/media  # Get chat media
```

### Messages
```bash
GET    /api/wa/sessions/{session_id}/messages/{chat_jid}    # Get chat messages
GET    /api/wa/sessions/{session_id}/messages/{message_id}  # Get specific message
DELETE /api/wa/sessions/{session_id}/messages/{message_id}  # Delete message
POST   /api/wa/sessions/{session_id}/messages/{message_id}/star   # Star message
POST   /api/wa/sessions/{session_id}/messages/{message_id}/unstar # Unstar message
```

### Message Status & Actions
```bash
POST   /api/wa/sessions/{session_id}/mark-read          # Mark messages as read
POST   /api/wa/sessions/{session_id}/mark-unread        # Mark messages as unread
POST   /api/wa/sessions/{session_id}/typing             # Send typing indicator
POST   /api/wa/sessions/{session_id}/presence           # Set presence status
GET    /api/wa/sessions/{session_id}/delivery-status    # Get message delivery status
```

## 🏷️ Group Management

### Group Operations
```bash
GET    /api/wa/sessions/{session_id}/groups                        # Get all groups
POST   /api/wa/sessions/{session_id}/groups                        # Create group
GET    /api/wa/sessions/{session_id}/groups/{group_jid}            # Get group details
PUT    /api/wa/sessions/{session_id}/groups/{group_jid}            # Update group info
DELETE /api/wa/sessions/{session_id}/groups/{group_jid}            # Delete group
POST   /api/wa/sessions/{session_id}/groups/{group_jid}/leave      # Leave group
```

### Group Members
```bash
GET    /api/wa/sessions/{session_id}/groups/{group_jid}/members    # Get group members
POST   /api/wa/sessions/{session_id}/groups/{group_jid}/members    # Add members
DELETE /api/wa/sessions/{session_id}/groups/{group_jid}/members/{jid} # Remove member
POST   /api/wa/sessions/{session_id}/groups/{group_jid}/admins/{jid}   # Promote to admin
DELETE /api/wa/sessions/{session_id}/groups/{group_jid}/admins/{jid}   # Demote admin
```

### Group Settings
```bash
GET    /api/wa/sessions/{session_id}/groups/{group_jid}/invite-code     # Get invite link
POST   /api/wa/sessions/{session_id}/groups/{group_jid}/invite-code     # Generate new invite
DELETE /api/wa/sessions/{session_id}/groups/{group_jid}/invite-code     # Revoke invite link
PUT    /api/wa/sessions/{session_id}/groups/{group_jid}/settings        # Update group settings
GET    /api/wa/sessions/{session_id}/groups/{group_jid}/picture         # Get group picture
POST   /api/wa/sessions/{session_id}/groups/{group_jid}/picture         # Set group picture
```

## 🔗 Webhook & Configuration

### Webhook Management
```bash
GET    /api/wa/webhook                     # Get webhook settings
POST   /api/wa/webhook                     # Set webhook URL
PUT    /api/wa/webhook                     # Update webhook settings
DELETE /api/wa/webhook                     # Remove webhook
GET    /api/wa/webhook/events              # Get supported events
POST   /api/wa/webhook/test                # Test webhook
```

### Session Configuration
```bash
GET    /api/wa/sessions/{session_id}/config     # Get session config
PUT    /api/wa/sessions/{session_id}/config     # Update session config
GET    /api/wa/sessions/{session_id}/proxy      # Get proxy settings
PUT    /api/wa/sessions/{session_id}/proxy      # Update proxy settings
GET    /api/wa/sessions/{session_id}/s3         # Get S3 media settings
PUT    /api/wa/sessions/{session_id}/s3         # Update S3 settings
```

## 📊 Analytics & Reporting

### Statistics
```bash
GET    /api/wa/sessions/{session_id}/stats              # Get session statistics
GET    /api/wa/sessions/{session_id}/message-stats      # Get message statistics
GET    /api/wa/sessions/{session_id}/usage              # Get usage statistics
GET    /api/wa/analytics/overview                       # Get analytics overview
GET    /api/wa/analytics/messages                       # Get message analytics
```

### Health & Monitoring
```bash
GET    /api/wa/health                      # Gateway health check
GET    /api/wa/sessions/{session_id}/ping  # Ping specific session
GET    /api/wa/server/status               # Get WA server status
GET    /api/wa/server/info                 # Get WA server information
```

## 🚫 Endpoint yang TIDAK Melalui Gateway

Endpoint-endpoint ini tetap direct ke server tanpa gateway:

### Admin Management (Direct ke Server WA)
```bash
GET    /admin/sessions         # Admin list sessions
POST   /admin/sessions         # Admin create session
DELETE /admin/sessions/{id}    # Admin delete session
GET    /admin/users            # Admin user management
```

### Health Check (Local)
```bash
GET    /health                 # Local app health
GET    /sessions/sync          # Session sync (internal)
```

## 🔐 Authentication untuk Gateway

Setiap request ke gateway harus menyertakan identifikasi user:

### Method 1: Headers
```bash
X-User-ID: user123
X-Session-ID: session123
```

### Method 2: Query Parameters
```bash
?user_id=user123&session_id=session123
```

### Method 3: API Key
```bash
Authorization: Bearer api_key_user
X-Session-ID: session123
```

## 📈 Rate Limiting Rules

### Message Endpoints
- **Limit**: 100 pesan per 60 detik (configurable)
- **Scope**: Per user_id + session_id combination
- **Response**: HTTP 429 dengan `Retry-After` header

### Other Endpoints
- **Session Management**: 10 requests per minute
- **Contact/Group Operations**: 20 requests per minute
- **Analytics/Stats**: 30 requests per minute

## 🎯 Migration Checklist

Untuk mengubah aplikasi client dari direct ke gateway:

1. ✅ **Update Base URL**
   ```
   http://wa-server:8080/ → http://gateway:8070/api/wa/
   ```

2. ✅ **Add Authentication Headers**
   ```bash
   # Tambahkan header
   X-User-ID: user_id_from_your_app
   X-Session-ID: session_id_from_your_app
   ```

3. ✅ **Remove Admin Token**
   ```bash
   # Hapus header ini (akan ditambahkan otomatis oleh gateway)
   Authorization: Bearer wa_admin_token
   ```

4. ✅ **Handle Rate Limit Response**
   ```javascript
   if (response.status === 429) {
     const retryAfter = response.headers['retry-after'];
     // Wait retryAfter seconds before retry
   }
   ```

5. ✅ **Update Error Handling**
   ```javascript
   // Handle gateway-specific errors
   if (response.status === 403) {
     // Subscription expired/inactive
   }
   if (response.status === 401) {
     // Authentication required
   }
   ```

## 📞 Support

Untuk pertanyaan atau issue terkait gateway implementation:

1. Check [Gateway Documentation](./GATEWAY_DOCUMENTATION.md)
2. Review [API Testing Guide](./API_TESTING_GATEWAY.md)
3. Monitor gateway logs: `docker compose logs -f genfity-wa-support`
4. Test health check: `curl http://localhost:8070/api/wa/health`
