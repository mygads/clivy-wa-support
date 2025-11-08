# Quick Start Guide - Campaign Management API

## Alur Lengkap Penggunaan API

### Prerequisites
- JWT Bearer Token untuk authentication user
- WhatsApp Session Token untuk akses ke server WhatsApp (diperlukan untuk contact sync)
- Base URL: `https://your-api-domain.com`

### Step 1: Contact Management

#### 1.1 Sync Contacts dari WhatsApp
```bash
POST /bulk/contact/sync
Authorization: Bearer YOUR_JWT_TOKEN
token: YOUR_WHATSAPP_SESSION_TOKEN
```

**Catatan:** Endpoint ini memerlukan **dua token**:
- JWT Bearer token untuk autentikasi user
- WhatsApp session token untuk mengakses data dari server WhatsApp

#### 1.2 Lihat Daftar Contacts
```bash
GET /bulk/contact
Authorization: Bearer YOUR_JWT_TOKEN
```

**Catatan:** Endpoint ini hanya perlu JWT Bearer token karena data diambil dari database lokal.

#### 1.3 Tambah Contact Manual (Opsional)
```bash
POST /bulk/contact/add
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "contacts": [
    {"phone": "628123456789", "full_name": "John Doe"}
  ]
}
```

### Step 2: Buat Campaign Template

#### 2.1 Text Campaign
```bash
POST /bulk/campaign
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "name": "Welcome Message",
  "type": "text",
  "message_body": "Selamat datang di layanan kami!"
}
```

#### 2.2 Image Campaign
```bash
POST /bulk/campaign
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "name": "Product Promo",
  "type": "image",
  "message_body": "Promo spesial bulan ini!",
  "image_url": "https://example.com/image.jpg",
  "caption": "Jangan lewatkan!"
}
```

### Step 3: Manage Campaign Templates

#### 3.1 Lihat Semua Templates
```bash
GET /bulk/campaign
Authorization: Bearer YOUR_JWT_TOKEN
```

#### 3.2 Lihat Template Spesifik
```bash
GET /bulk/campaign/{id}
Authorization: Bearer YOUR_JWT_TOKEN
```

#### 3.3 Update Template
```bash
PUT /bulk/campaign/{id}
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "name": "Updated Name",
  "message_body": "Updated message"
}
```

#### 3.4 Hapus Template
```bash
DELETE /bulk/campaign/{id}
Authorization: Bearer YOUR_JWT_TOKEN
```

### Step 4: Execute Bulk Campaign

#### 4.1 Kirim Sekarang
```bash
POST /bulk/campaign/execute
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "campaign_id": 1,
  "name": "Welcome Batch 1",
  "phone": ["628123456789", "628987654321"],
  "send_sync": "now"
}
```

#### 4.2 Jadwalkan Pengiriman
```bash
POST /bulk/campaign/execute
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "campaign_id": 1,
  "name": "Welcome Batch 2", 
  "phone": ["628123456789"],
  "send_sync": "2025-09-04 09:00:00"
}
```

### Step 5: Monitor Bulk Campaigns

#### 5.1 Lihat Semua Bulk Campaigns
```bash
GET /bulk/campaigns
Authorization: Bearer YOUR_JWT_TOKEN
```

#### 5.2 Detail Bulk Campaign + Status per Nomor
```bash
GET /bulk/campaigns/{id}
Authorization: Bearer YOUR_JWT_TOKEN
```

## Format Send Sync

| Format | Contoh | Keterangan |
|--------|--------|------------|
| Immediate | `"now"` | Langsung kirim |
| DateTime | `"2025-09-04 09:00:00"` | Tanggal + waktu spesifik |
| Date Only | `"2025-09-04"` | Tanggal saja (jam 9 pagi) |
| Time Only | `"09:00"` | Waktu saja (hari ini/besok) |

## Status Tracking

### Campaign Status
- `active` → Template siap digunakan
- `inactive` → Template tidak aktif
- `archived` → Template diarsipkan

### Bulk Campaign Status  
- `pending` → Akan dieksekusi segera
- `scheduled` → Dijadwalkan
- `processing` → Sedang diproses
- `completed` → Selesai
- `failed` → Gagal

### Item Status (per nomor)
- `pending` → Menunggu
- `sent` → Berhasil dikirim  
- `failed` → Gagal kirim

## Response Examples

### Success Response
```json
{
  "code": 200,
  "success": true,
  "message": "Operation successful",
  "data": { /* result data */ }
}
```

### Error Response
```json
{
  "code": 400,
  "success": false,
  "message": "Error description"
}
```

## Tips Penggunaan

1. **Sync Contacts Dulu**: Pastikan sudah sync contacts sebelum membuat campaign
2. **Template Reusable**: Buat template sekali, gunakan berkali-kali untuk bulk campaigns berbeda
3. **Monitor Progress**: Gunakan endpoint detail untuk tracking status pengiriman per nomor
4. **Scheduling**: Manfaatkan fitur scheduling untuk campaign yang direncanakan
5. **Error Handling**: Cek response code dan message untuk debugging

## Troubleshooting

| Error | Solusi |
|-------|--------|
| 401 Unauthorized | Periksa JWT token |
| 404 Not Found | ID campaign/bulk campaign tidak ditemukan |
| 400 Bad Request | Periksa format request body |
| 500 Internal Server Error | Cek server logs |
