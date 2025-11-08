# Campaign Management API Documentation

## Overview
API ini menyediakan sistem campaign management lengkap untuk WhatsApp messaging dengan fitur contact sync, template campaign, dan bulk messaging. Semua endpoint menggunakan JWT Bearer token authentication.

## Authentication
Semua endpoint memerlukan JWT Bearer token dalam header:
```
Authorization: Bearer <your-jwt-token>
```

## Complete Workflow

### 1. Contact Management

#### A. Sync Contacts dari WhatsApp Server
Sinkronisasi contacts dari external WhatsApp server ke database lokal.

**Endpoint:** `POST /bulk/contact/sync`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
token: <whatsapp-session-token>
Content-Type: application/json
```

**Request:** No body required

**Catatan Penting:** 
- Endpoint ini memerlukan **dua token berbeda**:
  1. **JWT Bearer token** di header `Authorization` untuk autentikasi user
  2. **WhatsApp session token** di header `token` untuk akses ke WhatsApp server
- JWT token digunakan untuk validasi user dan ownership
- WhatsApp session token digunakan untuk mengambil data contacts dari server WhatsApp eksternal

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "message": "Contact sync completed successfully",
  "data": {
    "total_contacts": 150,
    "new_contacts": 25,
    "updated_contacts": 125,
    "sync_source": "whatsapp_server"
  }
}
```

**Response Error (401):**
```json
{
  "code": 401,
  "success": false,
  "message": "User ID not found"
}
```

#### B. Get Contact List
Mendapatkan daftar semua contacts yang telah disync untuk user ini.

**Endpoint:** `GET /bulk/contact`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
```

**Catatan Penting:**
- Endpoint ini hanya memerlukan JWT Bearer token karena data diambil dari database lokal
- Tidak perlu WhatsApp session token karena tidak mengakses server WhatsApp eksternal
- Contact list yang dikembalikan adalah milik user yang terautentikasi saja

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "data": [
    {
      "phone": "628123456789",
      "full_name": "John Doe",
      "source": "sync"
    }
  ]
}
```

#### C. Add Contacts Manually
Menambahkan contacts secara manual ke database.

**Endpoint:** `POST /bulk/contact/add`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
Content-Type: application/json
```

**Request Body:**
```json
{
  "contacts": [
    {
      "phone": "628123456789",
      "full_name": "John Doe"
    },
    {
      "phone": "628987654321", 
      "full_name": "Jane Smith"
    }
  ]
}
```

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "message": "Processed 2 contacts (1 added, 1 updated)",
  "data": {
    "added_count": 1,
    "updated_count": 1,
    "contacts": [
      {
        "id": 1,
        "user_id": "user123",
        "phone": "628123456789",
        "full_name": "John Doe",
        "source": "manual"
      }
    ]
  }
}
```

#### D. Delete Contacts (Bulk)
Menghapus multiple contacts berdasarkan nomor telepon.

**Endpoint:** `DELETE /bulk/contact/delete`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
Content-Type: application/json
```

**Request Body:**
```json
{
  "phone": [
    "628123456789",
    "628987654321",
    "628555666777"
  ]
}
```

**Catatan Penting:**
- Hanya contacts milik user yang terautentikasi yang bisa dihapus
- Jika nomor telepon tidak ditemukan atau bukan milik user, akan diabaikan
- Response akan menunjukkan jumlah contacts yang berhasil dihapus

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "message": "Successfully deleted 3 contacts",
  "data": {
    "deleted_count": 3,
    "requested_count": 3
  }
}
```

**Response Success (Partial) (200):**
```json
{
  "code": 200,
  "success": true,
  "message": "Successfully deleted 2 contacts",
  "data": {
    "deleted_count": 2,
    "requested_count": 3
  }
}
```

**Response Error (404):**
```json
{
  "code": 404,
  "success": false,
  "message": "No contacts found for the provided phone numbers"
}
```

### 2. Campaign Template Management

#### A. Create Campaign Template
Membuat template campaign yang bisa digunakan berulang kali.

**Endpoint:** `POST /bulk/campaign`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
Content-Type: application/json
```

**Request Body untuk Text Campaign:**
```json
{
  "name": "Welcome Message",
  "type": "text",
  "message_body": "Selamat datang di layanan kami! Terima kasih telah bergabung."
}
```

**Request Body untuk Image Campaign:**
```json
{
  "name": "Product Promotion",
  "type": "image",
  "message_body": "Promo spesial bulan ini!",
  "image_url": "https://example.com/promo.jpg",
  "caption": "Jangan lewatkan kesempatan emas ini!"
}
```

**Response Success (201):**
```json
{
  "code": 201,
  "success": true,
  "message": "Campaign created successfully",
  "data": {
    "id": 1,
    "user_id": "user123",
    "name": "Welcome Message",
    "type": "text",
    "status": "active",
    "message_body": "Selamat datang di layanan kami! Terima kasih telah bergabung.",
    "image_url": "",
    "image_base64": "",
    "caption": "",
    "created_at": "2025-09-03T10:00:00Z",
    "updated_at": "2025-09-03T10:00:00Z"
  }
}
```

#### B. Get Campaign Templates
Mendapatkan semua campaign templates milik user.

**Endpoint:** `GET /bulk/campaign`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
```

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "data": [
    {
      "id": 1,
      "user_id": "user123",
      "name": "Welcome Message",
      "type": "text",
      "status": "active",
      "message_body": "Selamat datang di layanan kami!",
      "created_at": "2025-09-03T10:00:00Z",
      "updated_at": "2025-09-03T10:00:00Z"
    },
    {
      "id": 2,
      "user_id": "user123", 
      "name": "Product Promotion",
      "type": "image",
      "status": "active",
      "message_body": "Promo spesial bulan ini!",
      "image_url": "https://example.com/promo.jpg",
      "caption": "Jangan lewatkan kesempatan emas ini!",
      "created_at": "2025-09-03T10:00:00Z",
      "updated_at": "2025-09-03T10:00:00Z"
    }
  ]
}
```

#### C. Get Specific Campaign Template
Mendapatkan detail campaign template tertentu.

**Endpoint:** `GET /bulk/campaign/{id}`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
```

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "data": {
    "id": 1,
    "user_id": "user123",
    "name": "Welcome Message",
    "type": "text",
    "status": "active",
    "message_body": "Selamat datang di layanan kami! Terima kasih telah bergabung.",
    "created_at": "2025-09-03T10:00:00Z",
    "updated_at": "2025-09-03T10:00:00Z"
  }
}
```

#### D. Update Campaign Template
Mengupdate campaign template yang sudah ada.

**Endpoint:** `PUT /bulk/campaign/{id}`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
Content-Type: application/json
```

**Request Body:**
```json
{
  "name": "Updated Welcome Message",
  "message_body": "Selamat datang di layanan terbaru kami!",
  "status": "active"
}
```

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "message": "Campaign updated successfully",
  "data": {
    "id": 1,
    "user_id": "user123",
    "name": "Updated Welcome Message", 
    "message_body": "Selamat datang di layanan terbaru kami!",
    "updated_at": "2025-09-03T11:00:00Z"
  }
}
```

#### E. Delete Campaign Template
Menghapus campaign template.

**Endpoint:** `DELETE /bulk/campaign/{id}`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
```

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "message": "Campaign deleted successfully"
}
```

### 3. Bulk Campaign Execution

#### A. Execute Bulk Campaign
Menjalankan bulk campaign dengan menggunakan template yang sudah dibuat.

**Endpoint:** `POST /bulk/campaign/execute`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
Content-Type: application/json
```

**Request Body untuk Immediate Execution:**
```json
{
  "campaign_id": 1,
  "name": "Welcome Campaign Batch 1",
  "phone": [
    "628123456789",
    "628987654321",
    "628555666777"
  ],
  "send_sync": "now"
}
```

**Request Body untuk Scheduled Execution:**
```json
{
  "campaign_id": 1,
  "name": "Welcome Campaign Batch 1",
  "phone": [
    "628123456789",
    "628987654321"
  ],
  "send_sync": "2025-09-04 09:00:00"
}
```

**Response Success (201):**
```json
{
  "code": 201,
  "success": true,
  "message": "Bulk campaign created successfully",
  "data": {
    "bulk_campaign_id": 1,
    "total_recipients": 3,
    "status": "pending",
    "scheduled_at": null
  }
}
```

**Response untuk Scheduled Campaign:**
```json
{
  "code": 201,
  "success": true,
  "message": "Bulk campaign created successfully",
  "data": {
    "bulk_campaign_id": 2,
    "total_recipients": 2,
    "status": "scheduled",
    "scheduled_at": "2025-09-04T09:00:00Z"
  }
}
```

### 4. Bulk Campaign Monitoring

#### A. Get All Bulk Campaigns
Mendapatkan semua bulk campaigns milik user.

**Endpoint:** `GET /bulk/campaigns`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
```

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "data": [
    {
      "id": 1,
      "user_id": "user123",
      "campaign_id": 1,
      "name": "Welcome Campaign Batch 1",
      "type": "text",
      "message_body": "Selamat datang di layanan kami!",
      "status": "completed",
      "total_count": 3,
      "sent_count": 3,
      "failed_count": 0,
      "scheduled_at": null,
      "processed_at": "2025-09-03T10:05:00Z",
      "completed_at": "2025-09-03T10:06:00Z",
      "created_at": "2025-09-03T10:00:00Z",
      "updated_at": "2025-09-03T10:06:00Z",
      "campaign": {
        "id": 1,
        "name": "Welcome Message",
        "type": "text"
      }
    },
    {
      "id": 2,
      "user_id": "user123",
      "campaign_id": 1,
      "name": "Welcome Campaign Batch 2",
      "status": "scheduled",
      "total_count": 2,
      "sent_count": 0,
      "failed_count": 0,
      "scheduled_at": "2025-09-04T09:00:00Z",
      "created_at": "2025-09-03T10:00:00Z"
    }
  ]
}
```

#### B. Get Bulk Campaign Detail
Mendapatkan detail bulk campaign dengan status pengiriman per nomor.

**Endpoint:** `GET /bulk/campaigns/{id}`

**Headers:**
```
Authorization: Bearer <your-jwt-token>
```

**Response Success (200):**
```json
{
  "code": 200,
  "success": true,
  "data": {
    "bulk_campaign": {
      "id": 1,
      "user_id": "user123",
      "campaign_id": 1,
      "name": "Welcome Campaign Batch 1",
      "type": "text",
      "message_body": "Selamat datang di layanan kami!",
      "status": "completed",
      "total_count": 3,
      "sent_count": 2,
      "failed_count": 1,
      "processed_at": "2025-09-03T10:05:00Z",
      "completed_at": "2025-09-03T10:06:00Z",
      "campaign": {
        "id": 1,
        "name": "Welcome Message"
      }
    },
    "items": [
      {
        "id": 1,
        "bulk_campaign_id": 1,
        "phone": "628123456789",
        "status": "sent",
        "message_id": "msg_123",
        "sent_at": "2025-09-03T10:05:30Z",
        "error_message": ""
      },
      {
        "id": 2,
        "bulk_campaign_id": 1,
        "phone": "628987654321",
        "status": "sent",
        "message_id": "msg_124",
        "sent_at": "2025-09-03T10:05:45Z",
        "error_message": ""
      },
      {
        "id": 3,
        "bulk_campaign_id": 1,
        "phone": "628555666777",
        "status": "failed",
        "message_id": "",
        "sent_at": null,
        "error_message": "Invalid phone number format"
      }
    ]
  }
}
```

## Send Sync Format Options

Parameter `send_sync` mendukung berbagai format:

1. **Immediate:** `"now"` - Langsung dieksekusi
2. **Date Time:** `"2025-09-04 09:00:00"` - Tanggal dan waktu spesifik  
3. **Date Only:** `"2025-09-04"` - Tanggal saja (akan dieksekusi jam 9 pagi)
4. **Time Only:** `"09:00"` atau `"09:00:00"` - Waktu saja (hari ini, atau besok jika sudah lewat)

## Campaign Status

### Campaign Template Status:
- `active` - Template aktif dan bisa digunakan
- `inactive` - Template tidak aktif
- `archived` - Template diarsipkan

### Bulk Campaign Status:
- `pending` - Menunggu eksekusi (immediate)
- `scheduled` - Dijadwalkan untuk eksekusi nanti
- `processing` - Sedang diproses
- `completed` - Selesai diproses
- `failed` - Gagal diproses

### Bulk Campaign Item Status:
- `pending` - Menunggu pengiriman
- `sent` - Berhasil dikirim
- `failed` - Gagal dikirim

## Error Responses

### 400 Bad Request
```json
{
  "code": 400,
  "success": false,
  "message": "Invalid request: [detail error]"
}
```

### 401 Unauthorized
```json
{
  "code": 401,
  "success": false,
  "message": "User ID not found"
}
```

### 404 Not Found
```json
{
  "code": 404,
  "success": false,
  "message": "Campaign not found"
}
```

### 500 Internal Server Error
```json
{
  "code": 500,
  "success": false,
  "message": "Failed to create campaign: [detail error]"
}
```

## Example Complete Workflow

```bash
# 1. Sync contacts dari WhatsApp server (memerlukan dua token)
curl -X POST "https://api.example.com/bulk/contact/sync" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "token: YOUR_WHATSAPP_SESSION_TOKEN"

# 2. Lihat daftar contacts (hanya perlu JWT)
curl -X GET "https://api.example.com/bulk/contact" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# 3. Tambah contacts secara manual (hanya perlu JWT)
curl -X POST "https://api.example.com/bulk/contact/add" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "contacts": [
      {"phone": "628123456789", "full_name": "John Doe"},
      {"phone": "628987654321", "full_name": "Jane Smith"}
    ]
  }'

# 4. Hapus contacts (bulk delete) (hanya perlu JWT)
curl -X DELETE "https://api.example.com/bulk/contact/delete" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "phone": ["628123456789", "628987654321"]
  }'

# 5. Buat campaign template (hanya perlu JWT)
curl -X POST "https://api.example.com/bulk/campaign" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Welcome Message",
    "type": "text",
    "message_body": "Selamat datang di layanan kami!"
  }'

# 6. Execute bulk campaign (hanya perlu JWT)
curl -X POST "https://api.example.com/bulk/campaign/execute" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": 1,
    "name": "Welcome Batch 1",
    "phone": ["628123456789", "628987654321"],
    "send_sync": "now"
  }'

# 7. Monitor bulk campaign detail (hanya perlu JWT)
curl -X GET "https://api.example.com/bulk/campaigns/1" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Notes

1. **Authentication:** Semua endpoint memerlukan JWT Bearer token yang valid untuk autentikasi user
2. **Dual Token System:** 
   - **JWT Bearer token** (`Authorization: Bearer <jwt>`) - untuk autentikasi dan autorisasi user
   - **WhatsApp session token** (`token: <whatsapp-token>`) - untuk akses ke server WhatsApp eksternal (hanya diperlukan untuk contact sync)
3. **User Ownership:** Setiap user hanya bisa mengakses campaign dan contact miliknya sendiri
4. **Contact Management:**
   - **Contact Sync:** Hanya endpoint `/bulk/contact/sync` yang memerlukan kedua token
   - **Contact CRUD:** Endpoint `/bulk/contact/add` dan `/bulk/contact/delete` hanya memerlukan JWT token
   - **Contact List:** Endpoint `/bulk/contact` untuk melihat semua contacts milik user
5. **Campaign Templates:** Dapat digunakan berulang kali untuk multiple bulk campaigns
6. **Scheduling:** Mendukung penjadwalan campaign untuk eksekusi di masa depan
7. **Monitoring:** Detail tracking per nomor telepon untuk monitoring hasil pengiriman
8. **Error Handling:** Comprehensive error responses untuk debugging dan monitoring
9. **Data Isolation:** JWT memastikan setiap user hanya bisa mengakses data milik mereka sendiri
