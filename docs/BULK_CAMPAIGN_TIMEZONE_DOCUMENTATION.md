# Bulk Campaign API with Timezone Support - Documentation

## 📋 Ringkasan Perubahan

### 1. **Timezone Support**
- ✅ Menambahkan field `timezone` ke model `BulkCampaign`
- ✅ Support UTC offset format (`+07:00`, `-05:00`, `+05:30`, dll)
- ✅ Support IANA timezone names (`Asia/Jakarta`, `America/New_York`, dll)
- ✅ Auto-convert datetime user ke UTC untuk storage
- ✅ Validasi timezone dan scheduled time

### 2. **Delete Bulk Campaign**
- ✅ Endpoint baru: `DELETE /bulk/campaigns/:id`
- ✅ Prevent deletion saat campaign sedang processing
- ✅ Cascade delete items automatically

### 3. **Public Cron Endpoint**
- ✅ Endpoint `/bulk/cron/process` sekarang public (no JWT required)
- ✅ Dapat dipanggil oleh cron job tanpa authentication

### 4. **Homepage Endpoint**
- ✅ Endpoint baru: `GET /` untuk cek server status
- ✅ Menampilkan nama server, waktu, dan info lainnya

---

## 🗂️ Database Schema Changes

### Model: `WhatsAppBulkCampaigns`

**Field Baru:**
```sql
ALTER TABLE "WhatsAppBulkCampaigns" 
ADD COLUMN "timezone" VARCHAR(255);

-- Set default for existing records
UPDATE "WhatsAppBulkCampaigns" 
SET "timezone" = '' 
WHERE "timezone" IS NULL;
```

**Complete Schema:**
```prisma
model WhatsAppBulkCampaign {
  id                        BigInt                     @id @default(autoincrement())
  user_id                   String
  campaign_id               BigInt?
  name                      String
  type                      String
  message_body              String?
  image_url                 String?
  image_base64              String?
  caption                   String?
  status                    String?                    @default("pending")
  total_count               BigInt?                    @default(0)
  sent_count                BigInt?                    @default(0)
  failed_count              BigInt?                    @default(0)
  scheduled_at              DateTime?                  @db.Timestamptz(6)
  timezone                  String?                    // NEW FIELD
  processed_at              DateTime?                  @db.Timestamptz(6)
  completed_at              DateTime?                  @db.Timestamptz(6)
  error_message             String?
  created_at                DateTime?                  @db.Timestamptz(6)
  updated_at                DateTime?                  @db.Timestamptz(6)
  deleted_at                DateTime?                  @db.Timestamptz(6)
}
```

---

## 🔌 API Endpoints

### 1. **POST /bulk/campaign/execute** (UPDATED)

Execute bulk campaign dengan timezone support.

#### Headers:
```
Authorization: Bearer {JWT_TOKEN}
Content-Type: application/json
```

#### Request Body - Execute Now:
```json
{
  "campaign_id": 1,
  "name": "Promo Flash Sale",
  "phone": [
    "6281234567890",
    "6289876543210"
  ],
  "send_sync": "now"
}
```

#### Request Body - Schedule dengan UTC Offset:
```json
{
  "campaign_id": 1,
  "name": "Promo Besok Pagi",
  "phone": [
    "6281234567890",
    "6289876543210"
  ],
  "send_sync": "2025-10-10 09:00:00",
  "timezone": "+07:00"
}
```

#### Request Body - Schedule dengan IANA Timezone:
```json
{
  "campaign_id": 1,
  "name": "Promo Singapore",
  "phone": [
    "6591234567"
  ],
  "send_sync": "2025-10-10 09:00:00",
  "timezone": "Asia/Singapore"
}
```

#### Response - Immediate:
```json
{
  "code": 201,
  "success": true,
  "message": "Bulk campaign created successfully",
  "data": {
    "bulk_campaign_id": 123,
    "total_recipients": 2,
    "status": "pending"
  }
}
```

#### Response - Scheduled:
```json
{
  "code": 201,
  "success": true,
  "message": "Bulk campaign created successfully",
  "data": {
    "bulk_campaign_id": 124,
    "total_recipients": 2,
    "status": "scheduled",
    "scheduled_at": "2025-10-10T02:00:00Z",
    "timezone": "+07:00"
  }
}
```

#### Supported Timezone Formats:

**UTC Offset (Recommended):**
```
+07:00  - WIB (Jakarta, Bangkok)
+08:00  - WITA (Makassar, Singapore, Beijing)
+09:00  - WIT (Jayapura, Tokyo, Seoul)
+05:30  - India
-05:00  - EST (New York)
-08:00  - PST (Los Angeles)
```

**IANA Timezone Names:**
```
Asia/Jakarta
Asia/Makassar
Asia/Jayapura
Asia/Singapore
America/New_York
Europe/London
```

#### Supported Date Formats:
```
2025-10-10 14:30:00     (YYYY-MM-DD HH:MM:SS)
2025-10-10 14:30        (YYYY-MM-DD HH:MM)
2025-10-10T14:30:00     (ISO format)
10/10/2025 14:30        (DD/MM/YYYY HH:MM)
10-10-2025 14:30        (DD-MM-YYYY HH:MM)
```

#### Error Responses:

**Missing Timezone:**
```json
{
  "code": 400,
  "success": false,
  "message": "Invalid send_sync or timezone: timezone is required for scheduled campaigns (e.g., '+07:00', '-05:00', or 'Asia/Jakarta')"
}
```

**Invalid Timezone Format:**
```json
{
  "code": 400,
  "success": false,
  "message": "Invalid send_sync or timezone: invalid UTC offset '+7': offset too short, expected format: +HH:MM or -HH:MM"
}
```

**Time in Past:**
```json
{
  "code": 400,
  "success": false,
  "message": "Invalid send_sync or timezone: scheduled time must be in the future (current time in +07:00: 2025-10-08 15:30:00)"
}
```

---

### 2. **GET /bulk/campaigns** (UPDATED)

List semua bulk campaigns dengan timezone info.

#### Headers:
```
Authorization: Bearer {JWT_TOKEN}
```

#### Response:
```json
{
  "code": 200,
  "success": true,
  "data": [
    {
      "id": 123,
      "user_id": "user_123",
      "campaign_id": 1,
      "name": "Promo Besok Pagi",
      "type": "text",
      "message_body": "Halo, ada promo spesial!",
      "status": "scheduled",
      "total_count": 2,
      "sent_count": 0,
      "failed_count": 0,
      "scheduled_at": "2025-10-10T02:00:00Z",
      "timezone": "+07:00",
      "processed_at": null,
      "completed_at": null,
      "created_at": "2025-10-08T08:30:00Z",
      "updated_at": "2025-10-08T08:30:00Z"
    },
    {
      "id": 122,
      "user_id": "user_123",
      "campaign_id": 1,
      "name": "Promo Flash Sale",
      "type": "text",
      "status": "completed",
      "total_count": 5,
      "sent_count": 5,
      "failed_count": 0,
      "scheduled_at": null,
      "timezone": "",
      "completed_at": "2025-10-08T08:25:00Z",
      "created_at": "2025-10-08T08:20:00Z",
      "updated_at": "2025-10-08T08:25:00Z"
    }
  ]
}
```

---

### 3. **GET /bulk/campaigns/:id** (UPDATED)

Detail bulk campaign dengan timezone info.

#### Headers:
```
Authorization: Bearer {JWT_TOKEN}
```

#### Response:
```json
{
  "code": 200,
  "success": true,
  "data": {
    "bulk_campaign": {
      "id": 123,
      "user_id": "user_123",
      "campaign_id": 1,
      "name": "Promo Besok Pagi",
      "type": "text",
      "message_body": "Halo, ada promo spesial!",
      "status": "scheduled",
      "total_count": 2,
      "sent_count": 0,
      "failed_count": 0,
      "scheduled_at": "2025-10-10T02:00:00Z",
      "timezone": "+07:00",
      "created_at": "2025-10-08T08:30:00Z",
      "updated_at": "2025-10-08T08:30:00Z"
    },
    "items": [
      {
        "id": 1,
        "bulk_campaign_id": 123,
        "phone": "6281234567890",
        "status": "pending",
        "message_id": "",
        "error_message": "",
        "sent_at": null
      },
      {
        "id": 2,
        "bulk_campaign_id": 123,
        "phone": "6289876543210",
        "status": "pending",
        "message_id": "",
        "error_message": "",
        "sent_at": null
      }
    ]
  }
}
```

---

### 4. **DELETE /bulk/campaigns/:id** (NEW!)

Delete bulk campaign.

#### Headers:
```
Authorization: Bearer {JWT_TOKEN}
```

#### Request:
```
DELETE /bulk/campaigns/123
```

#### Success Response:
```json
{
  "code": 200,
  "success": true,
  "message": "Bulk campaign deleted successfully"
}
```

#### Error - Campaign Processing:
```json
{
  "code": 400,
  "success": false,
  "message": "Cannot delete campaign that is currently processing"
}
```

#### Error - Not Found:
```json
{
  "code": 404,
  "success": false,
  "message": "Bulk campaign not found"
}
```

---

### 5. **GET /bulk/cron/process** (UPDATED - Now Public)

Process scheduled campaigns (untuk cron job).

**PERUBAHAN:** Endpoint ini sekarang **PUBLIC** dan tidak memerlukan JWT token lagi.

#### Request:
```bash
curl http://localhost:8070/bulk/cron/process
```

#### Response:
```json
{
  "code": 200,
  "success": true,
  "message": "Bulk campaign cron job completed",
  "data": {
    "processed_count": 3,
    "checked_at": "2025-10-08T15:30:00Z"
  }
}
```

#### Cron Job Setup (Linux/Mac):
```bash
# Edit crontab
crontab -e

# Add line to run every minute
* * * * * curl http://localhost:8070/bulk/cron/process
```

#### Cron Job Setup (Windows Task Scheduler):
```
Program: curl.exe
Arguments: http://localhost:8070/bulk/cron/process
Trigger: Repeat every 1 minute
```

---

### 6. **GET /** (NEW!)

Homepage untuk cek server status.

#### Request:
```bash
curl http://localhost:8070/
```

#### Response:
```json
{
  "status": "running",
  "server": "Genfity WhatsApp Support API",
  "service": "genfity-wa-support",
  "version": "1.0.1",
  "time": "2025-10-08 22:30:15",
  "timezone": "WIB",
  "timestamp": 1728410415,
  "message": "Server is running successfully",
  "environment": "release"
}
```

#### Customize Server Name (.env):
```env
SERVER_NAME=Genfity WhatsApp Production Server
```

---

## 🌍 Timezone Reference

### Indonesia Timezones

| Region | UTC Offset | IANA Name | Kota |
|--------|-----------|-----------|------|
| WIB | `+07:00` | `Asia/Jakarta` | Jakarta, Bandung, Medan, Palembang |
| WITA | `+08:00` | `Asia/Makassar` | Makassar, Denpasar, Balikpapan |
| WIT | `+09:00` | `Asia/Jayapura` | Jayapura, Manokwari, Sorong |

### Asia Timezones

| Country | UTC Offset | Cities |
|---------|-----------|--------|
| Thailand | `+07:00` | Bangkok |
| Singapore | `+08:00` | Singapore |
| Malaysia | `+08:00` | Kuala Lumpur |
| Philippines | `+08:00` | Manila |
| China | `+08:00` | Beijing, Shanghai |
| Japan | `+09:00` | Tokyo, Osaka |
| South Korea | `+09:00` | Seoul |
| India | `+05:30` | Mumbai, Delhi |
| Pakistan | `+05:00` | Karachi, Lahore |
| UAE | `+04:00` | Dubai, Abu Dhabi |

### Other Regions

| Region | UTC Offset | Cities |
|--------|-----------|--------|
| USA East | `-05:00` | New York, Washington DC |
| USA West | `-08:00` | Los Angeles, San Francisco |
| UK | `+00:00` | London |
| Europe Central | `+01:00` | Paris, Berlin, Rome |
| Australia East | `+10:00` | Sydney, Melbourne |

---

## 🔄 Cara Kerja Timezone

### Flow Diagram:

```
User Input → Parse Timezone → Convert to UTC → Store to DB → Cron Check → Execute

Example:
1. User: "2025-10-10 14:30:00" + timezone "+07:00"
2. System Parse: 14:30 in UTC+07:00
3. Convert to UTC: 07:30 UTC (14:30 - 7 hours)
4. Store: scheduled_at = "2025-10-10T07:30:00Z", timezone = "+07:00"
5. Cron: Check if current_time_utc >= scheduled_at_utc
6. Execute: Send messages when time arrives
```

### UTC Conversion Examples:

**WIB (+07:00):**
```
User Time:    2025-10-10 14:30:00 +07:00
UTC Time:     2025-10-10 07:30:00 UTC
Calculation:  14:30 - 7 = 07:30
```

**WITA (+08:00):**
```
User Time:    2025-10-10 14:30:00 +08:00
UTC Time:     2025-10-10 06:30:00 UTC
Calculation:  14:30 - 8 = 06:30
```

**EST (-05:00):**
```
User Time:    2025-10-10 09:00:00 -05:00
UTC Time:     2025-10-10 14:00:00 UTC
Calculation:  09:00 + 5 = 14:00
```

**India (+05:30):**
```
User Time:    2025-10-10 16:00:00 +05:30
UTC Time:     2025-10-10 10:30:00 UTC
Calculation:  16:00 - 5:30 = 10:30
```

---

## 🧪 Testing Examples

### Test 1: Execute Now (No Timezone)
```bash
curl -X POST http://localhost:8070/bulk/campaign/execute \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": 1,
    "name": "Test Immediate",
    "phone": ["6281234567890"],
    "send_sync": "now"
  }'
```

### Test 2: Schedule WIB (+07:00)
```bash
curl -X POST http://localhost:8070/bulk/campaign/execute \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": 1,
    "name": "Test WIB",
    "phone": ["6281234567890"],
    "send_sync": "2025-10-10 14:30:00",
    "timezone": "+07:00"
  }'
```

### Test 3: Schedule WITA (+08:00)
```bash
curl -X POST http://localhost:8070/bulk/campaign/execute \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": 1,
    "name": "Test WITA",
    "phone": ["6281234567890"],
    "send_sync": "2025-10-10 14:30:00",
    "timezone": "+08:00"
  }'
```

### Test 4: Schedule India (+05:30)
```bash
curl -X POST http://localhost:8070/bulk/campaign/execute \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": 1,
    "name": "Test India",
    "phone": ["919876543210"],
    "send_sync": "2025-10-10 16:00:00",
    "timezone": "+05:30"
  }'
```

### Test 5: Schedule dengan IANA Timezone
```bash
curl -X POST http://localhost:8070/bulk/campaign/execute \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": 1,
    "name": "Test IANA",
    "phone": ["6281234567890"],
    "send_sync": "2025-10-10 14:30:00",
    "timezone": "Asia/Jakarta"
  }'
```

### Test 6: Delete Bulk Campaign
```bash
curl -X DELETE http://localhost:8070/bulk/campaigns/123 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Test 7: Check Server Status
```bash
curl http://localhost:8070/
```

### Test 8: Cron Job (Public)
```bash
curl http://localhost:8070/bulk/cron/process
```

---

## ⚠️ Important Notes

### 1. **Timezone adalah Wajib untuk Schedule**
- Execute now: timezone TIDAK perlu
- Execute later: timezone WAJIB diisi

### 2. **Format Timezone**
- **Recommended:** UTC offset (`+07:00`, `-05:00`)
- **Alternative:** IANA names (`Asia/Jakarta`)

### 3. **Scheduled Time Storage**
- Semua scheduled time disimpan dalam **UTC** di database
- Timezone user disimpan terpisah untuk reference

### 4. **Cron Job**
- Endpoint `/bulk/cron/process` sekarang **public**
- Tidak perlu JWT token
- Jalankan setiap 1 menit

### 5. **Delete Protection**
- Campaign yang sedang `processing` tidak bisa dihapus
- Campaign `pending`, `scheduled`, `completed`, dan `failed` bisa dihapus

---

## 📝 Migration Guide

### From Old Version (No Timezone):
```sql
-- Add timezone column
ALTER TABLE "WhatsAppBulkCampaigns" 
ADD COLUMN "timezone" VARCHAR(255);

-- Set empty string for existing records
UPDATE "WhatsAppBulkCampaigns" 
SET "timezone" = '' 
WHERE "timezone" IS NULL;
```

### Update Your Client Code:
```javascript
// OLD WAY (masih work, tapi tidak ada timezone)
{
  "send_sync": "now"
}

// NEW WAY (dengan timezone support)
{
  "send_sync": "2025-10-10 14:30:00",
  "timezone": "+07:00"  // Add this for scheduled campaigns
}
```

---

## 🎯 Benefits

1. **Clear Timezone Info**: User tahu persis kapan campaign akan dijalankan
2. **No Ambiguity**: Tidak ada kebingungan timezone
3. **Universal**: Support semua timezone di dunia
4. **Flexible**: Support UTC offset dan IANA names
5. **Future-proof**: Prepared untuk DST (Daylight Saving Time)
6. **Better UX**: Error messages yang jelas dengan current time di timezone user

---

## 🔗 Related Files

- `models/campaign.go` - Model definitions
- `handlers/campaign.go` - API handlers
- `main.go` - Route definitions
- `database/database.go` - Database configuration

---

## 📞 Support

Untuk pertanyaan atau issue:
1. Check error message yang detailed
2. Verify timezone format (`+HH:MM` atau IANA name)
3. Check scheduled time tidak di masa lalu
4. Monitor cron job logs

---

**Last Updated:** October 8, 2025  
**API Version:** 1.0.1  
**Author:** Genfity Development Team
