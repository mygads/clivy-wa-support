# Dokumentasi Lengkap WhatsApp Webhook API

## Format Webhook yang Didukung

### 1. Menerima Pesan (Message)

Format webhook saat menerima pesan memiliki struktur:
- `event.Info` - berisi metadata pesan (ID, Sender, Chat, Timestamp, dll)
- `event.Message` - berisi konten pesan (conversation, extendedTextMessage, dll)
- `token` - token session WhatsApp
- `type` - jenis event ("Message")

Contoh:
```json
{
  "event": {
    "Info": {
      "ID": "53829C3632356D0F08BFB6BDB41A706F",
      "Sender": "6289668176764@s.whatsapp.net",
      "Chat": "6281233784490@s.whatsapp.net",
      "IsFromMe": true,
      "IsGroup": false,
      "PushName": "Yoga",
      "Type": "text",
      "Timestamp": "2025-08-14T14:55:01+07:00"
    },
    "Message": {
      "conversation": "ya"
    }
  },
  "token": "genfitywa1",
  "type": "Message"
}
```

**Penting**: 
- `6289668176764` adalah pemilik token/session WA
- `6281233784490` adalah orang lain yang mengirim pesan ke pemilik token

### 2. Status Pesan (ReadReceipt)

Format untuk status terkirim/dibaca:
```json
{
  "event": {
    "Chat": "6281233784490@s.whatsapp.net",
    "Sender": "6281233784490:24@s.whatsapp.net",
    "MessageIDs": ["53829C3632356D0F08BFB6BDB41A706F"],
    "Timestamp": "2025-08-14T14:55:54+07:00",
    "Type": "read"
  },
  "state": "Delivered", // atau "Read"
  "token": "genfitywa1",
  "type": "ReadReceipt"
}
```

### 3. Status Mengetik (ChatPresence)

```json
{
  "event": {
    "Chat": "6281233784490@s.whatsapp.net",
    "Sender": "6281233784490@s.whatsapp.net",
    "State": "composing" // atau "paused"
  },
  "token": "genfitywa1",
  "type": "ChatPresence"
}
```

**Fitur Auto-Stop**: Jika status composing tidak diikuti dengan paused dalam 10 detik, sistem otomatis mengubah status menjadi paused.

### 4. Session Connected

```json
{
  "event": {
    "Action": {"name": "~"},
    "FromFullSync": false,
    "Timestamp": "2025-08-14T14:55:41.474+07:00"
  },
  "token": "genfitywa1",
  "type": "Connected"
}
```

### 5. QR Code Login

```json
{
  "event": "code",
  "qrCodeBase64": "data:image/png;base64,iVBORw0...",
  "token": "genfitywa1",
  "type": "QR"
}
```

**QR Management**: 
- QR disimpan dengan expired 60 detik
- Ketika connected, QR dihapus otomatis
- QR baru akan menggantikan yang lama

## API Endpoints

### Webhook Endpoints
- `POST /webhook/wa` - Menerima webhook dari server WhatsApp

### Data Retrieval
- `GET /api/v1/messages` - Daftar pesan dengan filter
- `GET /api/v1/message-statuses` - Status delivery/read pesan
- `GET /api/v1/chat-presences` - Status typing/composing
- `GET /api/v1/sessions` - Daftar session WhatsApp
- `GET /api/v1/sessions/:token/qr` - QR code untuk session tertentu

### Session Management
- `GET /api/v1/sessions/sync` - **Trigger sync dengan server WA**
- `GET /api/v1/events` - Daftar semua webhook events
- `GET /api/v1/users` - Daftar user token aktif

## Proses Penanganan Data

### 1. Menerima Pesan
1. Parse struktur webhook baru (`event.Info` dan `event.Message`)
2. Extract informasi pesan (ID, sender, content, timestamp)
3. Simpan ke database `whatsapp_messages`
4. Support untuk text, media, location, contact

### 2. Status Typing
1. Ketika menerima status "composing", set timer 10 detik
2. Jika tidak ada event "paused" dalam 10 detik, auto-stop
3. Update field `auto_stopped = true`

### 3. Status Pesan
1. Update tabel `whatsapp_message_statuses`
2. Update status di tabel utama `whatsapp_messages`
3. Support untuk "delivered" dan "read"

### 4. Session Management
1. Monitor status dari server WA via API `/admin/users`
2. Sync status connected/disconnected
3. Manage QR codes dengan expiration

## Database Schema

### Tabel Utama:
- `whatsapp_messages` - Pesan utama
- `whatsapp_sessions` - Status session & QR
- `whatsapp_message_statuses` - Status delivery/read
- `whatsapp_chat_presences` - Status typing
- `gen_event_webhooks` - Raw webhook events

### Field Penting:
- `user_token` - Identifier session WA
- `message_id` - ID unik pesan
- `from`/`to` - Pengirim/penerima
- `session_state` - connecting/qr_waiting/connected/disconnected

## Sync dengan Server WhatsApp

### Endpoint Trigger: `GET /api/v1/sessions/sync`

Proses:
1. Request ke `{WA_SERVER_URL}/admin/users` dengan header `Authorization`
2. Parse response untuk semua user sessions
3. Update database lokal dengan status terbaru
4. Manage QR codes dan connection status

### Environment Variables:
```
WA_SERVER_URL=http://localhost:8080
WA_ADMIN_TOKEN=your_admin_token_here
```

## Fitur Khusus

### 1. Multi-Session Support
- Setiap `token` mewakili session WA terpisah
- Bisa handle multiple WhatsApp accounts

### 2. Message Filtering
- Filter pesan kosong
- Filter berdasarkan keyword/sender
- Configurable di `shouldFilterMessage()`

### 3. Auto-Expiration
- QR codes expired otomatis (60 detik)
- Typing status auto-stopped (10 detik)

### 4. Backward Compatibility
- Mendukung format webhook lama
- Fallback untuk field yang hilang

## Testing & Monitoring

### Health Check:
- `GET /api/v1/health` - Status aplikasi

### Debugging:
- Raw webhook data disimpan di `gen_event_webhooks.raw_data`
- Log detail di console untuk troubleshooting

### Pagination:
- Semua endpoint list mendukung `page` dan `limit`
- Default: page=1, limit=10

## Cron Job Integration

Untuk monitoring otomatis, setup cron job yang memanggil:
```bash
curl -X GET "http://your-api-url/api/v1/sessions/sync"
```

Recommended interval: setiap 30-60 detik untuk real-time monitoring.

## Error Handling

- Webhook parsing errors tidak menginterrupt proses
- Failed message processing di-log tapi tidak stop aplikasi
- Graceful fallback untuk field yang tidak ada
- Retry mechanism untuk external API calls

Sistem ini dirancang untuk production-ready dengan fokus pada:
- **Reliability**: Handle berbagai format webhook
- **Scalability**: Support multiple sessions
- **Monitoring**: Real-time status tracking
- **Maintenance**: Easy debugging dan monitoring
