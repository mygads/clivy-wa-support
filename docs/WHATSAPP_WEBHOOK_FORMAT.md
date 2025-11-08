# WhatsApp Webhook Format Documentation

## Perubahan yang Dilakukan Penting

### Model Database Baru

1. **WhatsAppSession** - Untuk mengelola status session WhatsApp
   - `session_state`: connecting, qr_waiting, connected, disconnected
   - `qr_code`: Base64 QR code data
   - `qr_expired_at`: Waktu kadaluarsa QR code
   - `connected_at`: Waktu terkoneksi
   - `disconnected_at`: Waktu terputus

2. **WhatsAppMessageStatus** - Untuk tracking status pesan (delivered/read)
   - `message_id`: ID pesan yang statusnya diupdate
   - `status`: delivered, read
   - `event_timestamp`: Waktu status berubah

3. **WhatsAppChatPresence** - Diperbaharui untuk auto-stop typing
   - `auto_stopped`: Flag jika auto-stopped setelah 10 detik
   - `expires_at`: Waktu ekspirasi status mengetik

### Endpoint API Baru

1. **GET /api/v1/sessions** - Daftar semua session WhatsApp
2. **GET /api/v1/sessions/:user_token/qr** - Mendapatkan QR code untuk session tertentu
3. **GET /api/v1/message-statuses** - Daftar status pesan (delivered/read)
4. **GET /api/v1/chat-presences** - Daftar status typing/composing

### Format Webhook yang Didukung

#### 1. Menerima Pesan (Message)
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

#### 2. Status Pesan Terkirim/Dibaca (ReadReceipt)
```json
{
  "event": {
    "Chat": "6281233784490@s.whatsapp.net",
    "Sender": "6281233784490:24@s.whatsapp.net",
    "MessageIDs": ["53829C3632356D0F08BFB6BDB41A706F"],
    "Timestamp": "2025-08-14T14:55:54+07:00",
    "Type": "read"
  },
  "state": "Read",
  "token": "genfitywa1",
  "type": "ReadReceipt"
}
```

#### 3. Session Connected
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

#### 4. QR Code untuk Login
```json
{
  "event": "code",
  "qrCodeBase64": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAQ...",
  "token": "genfitywa1",
  "type": "QR"
}
```

#### 5. Status Typing (ChatPresence)
```json
{
  "event": {
    "Chat": "6281233784490@s.whatsapp.net",
    "Sender": "6281233784490@s.whatsapp.net",
    "State": "composing"
  },
  "token": "genfitywa1",
  "type": "ChatPresence"
}
```

### Fitur Auto-Stop Typing

- Ketika ada event `composing`, sistem akan otomatis mengubah status menjadi `paused` setelah 10 detik jika tidak ada event `paused` yang diterima
- Field `auto_stopped` akan di-set `true` jika status diubah otomatis
- Field `expires_at` menyimpan waktu kapan status composing akan expired

### Proses Penanganan Webhook

1. **Menerima Message**: Pesan disimpan ke database dengan parsing struktur yang baru
2. **Status Delivered/Read**: Update status pesan di database dan create record message status
3. **Session Management**: Update status session (connecting, qr_waiting, connected)
4. **QR Code**: Simpan QR code dengan waktu expired (60 detik)
5. **Typing Status**: Simpan status typing dengan auto-stop mechanism

### Catatan Penting

1. **User Token**: Setiap webhook memiliki `token` yang mengidentifikasi session WhatsApp
2. **Backward Compatibility**: Masih mendukung format webhook lama
3. **Auto Migration**: Database akan otomatis membuat tabel baru saat aplikasi dijalankan
4. **Filtering**: Pesan kosong dan spam dapat difilter
5. **Pagination**: Semua endpoint mendukung pagination

### Contoh Penggunaan API

#### Mendapatkan QR Code untuk Login
```bash
GET /api/v1/sessions/genfitywa1/qr
```

#### Mendapatkan Status Pesan
```bash
GET /api/v1/message-statuses?user_token=genfitywa1&message_id=53829C3632356D0F08BFB6BDB41A706F
```

#### Mendapatkan Status Typing
```bash
GET /api/v1/chat-presences?user_token=genfitywa1&chat_jid=6281233784490@s.whatsapp.net&state=composing
```
