# Clivy Chat AI - WhatsApp Event API & Gateway

A comprehensive WhatsApp API gateway and event processing system with JWT authentication, subscription validation, image processing, contact management, and campaign-based bulk messaging capabilities.

## Features

### Core Gateway Features
- ✅ **API Gateway** - Proxy requests to WhatsApp server with validation
- ✅ **Subscription Validation** - Check user subscription status  
- ✅ **JWT Authentication** - Bearer token authentication for user ownership
- ✅ **Session Management** - Validate and track WhatsApp sessions
- ✅ **Message Tracking** - Track message quotas and limits
- ✅ **Image Processing** - URL to base64 conversion for images
- ✅ **Admin Bypass** - Admin endpoints bypass all validation

### Contact Management
- ✅ **Contact Sync** - Sync contacts from WhatsApp server
- ✅ **Contact List** - Get simplified contact list
- ✅ **Database Storage** - Store contacts with user ownership
- ✅ **Manual Contact Addition** - Add contacts manually

### Campaign-Based Messaging System
- ✅ **Campaign Templates** - Create reusable message templates owned by users
- ✅ **Bulk Campaign Execution** - Execute campaigns to multiple recipients
- ✅ **Message Scheduling** - Schedule campaigns for future delivery
- ✅ **Status Tracking** - Track individual message delivery status
- ✅ **Template Management** - CRUD operations for campaign templates
- ✅ **User Ownership** - Each user can only access their own campaigns
- ✅ **Cron Job Support** - Process scheduled campaigns automatically

### Event Processing
- ✅ **Webhook Handling** - Receive and process WhatsApp events
- ✅ **Event Storage** - Store all events with proper categorization
- ✅ **Real-time Processing** - Process events in real-time
- ✅ **Event Types** - Support for messages, receipts, presence, etc.

## 📖 Complete Documentation

### Main Documentation
- **[Campaign API Documentation](./CAMPAIGN_API_DOCUMENTATION.md)** - Complete API reference with examples
- **[Quick Start Guide](./QUICK_START_GUIDE.md)** - Step-by-step usage guide
- **[Usage Examples](./USAGE_EXAMPLES.md)** - JavaScript/Python code examples

### Additional Documentation
- **[Contact Sync API](./CONTACT_SYNC_API.md)** - Contact management endpoints
- **[Webhook Documentation](./WEBHOOK_DOCUMENTATION.md)** - Webhook handling
- **[Testing Guide](./API_TESTING_GUIDE.md)** - API testing examples

## Quick Start

### 1. Environment Setup
```bash
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=wa-support_db
DB_SSLMODE=disable

# Transactional Database
TRANSACTIONAL_DB_HOST=localhost
TRANSACTIONAL_DB_PORT=5432
TRANSACTIONAL_DB_USER=postgres
TRANSACTIONAL_DB_PASSWORD=password
TRANSACTIONAL_DB_NAME=transactional_db
TRANSACTIONAL_DB_SSLMODE=disable

# WhatsApp Server
WHATSAPP_SERVER_URL=https://wa.genfity.com

# JWT Configuration
JWT_SECRET=your-secret-key-here

# Gateway Configuration
GATEWAY_MODE=production
PORT=8070
```

### 2. Run the Server
```bash
go run main.go
```

### 3. Complete Workflow Example

#### Authentication
All requests require JWT Bearer token:
```bash
Authorization: Bearer <your-jwt-token>
```

#### Step 1: Sync Contacts
```bash
curl -X POST http://localhost:8070/bulk/contact/sync \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "token: YOUR_WHATSAPP_SESSION_TOKEN"
```

**Note:** Contact sync requires both JWT Bearer token (for user authentication) and WhatsApp session token (for accessing WhatsApp server).

#### Step 2: Create Campaign Template
```bash
curl -X POST http://localhost:8070/bulk/campaign \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Welcome Message",
    "type": "text", 
    "message_body": "Selamat datang di layanan kami!"
  }'
```

#### Step 3: Execute Bulk Campaign
```bash
curl -X POST http://localhost:8070/bulk/campaign/execute \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "campaign_id": 1,
    "name": "Welcome Campaign Batch 1",
    "phone": ["628123456789", "628987654321"],
    "send_sync": "now"
  }'
```

#### Step 4: Monitor Campaign Progress
```bash
curl -X GET http://localhost:8070/bulk/campaigns/1 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## API Endpoints Overview

### Authentication Required (Bearer Token)
All `/bulk/*` endpoints require JWT authentication.

### Contact Management
```
POST /bulk/contact/sync         - Sync contacts from WhatsApp server
GET  /bulk/contact              - Get user's contacts
POST /bulk/contact/add          - Add contacts manually
```

### Campaign Templates
```
POST   /bulk/campaign           - Create campaign template
GET    /bulk/campaign           - Get user's campaigns
GET    /bulk/campaign/{id}      - Get specific campaign
PUT    /bulk/campaign/{id}      - Update campaign
DELETE /bulk/campaign/{id}      - Delete campaign
```

### Bulk Campaign Execution
```
POST /bulk/campaign/execute     - Execute bulk campaign
GET  /bulk/campaigns            - Get user's bulk campaigns
GET  /bulk/campaigns/{id}       - Get bulk campaign details
GET  /bulk/cron/process         - Process scheduled campaigns
```

### Gateway Routes (Token Header)
```
/wa/admin/*     - Admin routes (no validation)
/wa/session/*   - Session management
/wa/webhook/*   - Webhook endpoints
/wa/chat/*      - Chat operations
/wa/user/*      - User operations
/wa/group/*     - Group operations
/wa/newsletter/* - Newsletter operations
```

## Database Schema

### User & Session Management
- `User` - User accounts with JWT authentication
- `UserSession` - JWT session management
- `WhatsAppSession` - WhatsApp session mapping

### Contact Management
- `WhatsAppContact` - User-owned contact database

### Campaign System
- `Campaign` - User-owned campaign templates
- `BulkCampaign` - Campaign execution records
- `BulkCampaignItem` - Individual message tracking

### Event Processing
- `gen_event_webhooks` - All webhook events
- `whats_app_messages` - Processed messages
- `chat_rooms` - Chat conversations
- `chat_messages` - Individual messages

## Campaign Workflow

1. **User Authentication** - JWT Bearer token validation
2. **Contact Sync** - Sync contacts from WhatsApp server to user's database
3. **Template Creation** - Create reusable campaign templates
4. **Campaign Execution** - Execute templates to selected contacts
5. **Scheduling Support** - Immediate or scheduled delivery
6. **Progress Monitoring** - Track delivery status per recipient
7. **User Ownership** - Each user manages their own campaigns and contacts

## Send Sync Formats

| Format | Example | Description |
|--------|---------|-------------|
| Immediate | `"now"` | Send immediately |
| DateTime | `"2025-09-04 09:00:00"` | Specific date and time |
| Date Only | `"2025-09-04"` | Date only (9 AM default) |
| Time Only | `"09:00"` | Time only (today/tomorrow) |

## Status Tracking

### Campaign Template Status
- `active` - Template ready for use
- `inactive` - Template disabled
- `archived` - Template archived

### Bulk Campaign Status
- `pending` - Queued for immediate execution
- `scheduled` - Scheduled for future execution
- `processing` - Currently being processed
- `completed` - Successfully completed
- `failed` - Execution failed

### Individual Message Status
- `pending` - Waiting to be sent
- `sent` - Successfully sent
- `failed` - Failed to send

## Error Handling

### HTTP Status Codes
- **200 OK** - Successful operation
- **201 Created** - Resource created successfully
- **400 Bad Request** - Invalid request format
- **401 Unauthorized** - Invalid or missing JWT token
- **403 Forbidden** - Subscription expired or limit exceeded
- **404 Not Found** - Resource not found
- **500 Internal Server Error** - Server error

### Error Response Format
```json
{
  "code": 400,
  "success": false,
  "message": "Error description"
}
```

## Security Features

- **JWT Authentication** - Secure user authentication
- **User Isolation** - Users can only access their own data
- **Token Validation** - Comprehensive JWT token validation
- **Session Management** - Secure session tracking
- **Input Validation** - Request validation and sanitization

## Performance Features

- **Silent GORM Logging** - Clean logs without query noise
- **Background Processing** - Async campaign execution
- **Batch Processing** - Efficient bulk operations
- **Database Indexing** - Optimized database queries
- **Connection Pooling** - Efficient database connections

## Monitoring & Observability

- **Campaign Progress Tracking** - Real-time progress monitoring
- **Individual Message Status** - Per-recipient delivery tracking
- **Error Logging** - Comprehensive error tracking
- **Performance Metrics** - Request timing and success rates
- **Health Check Endpoint** - Server status monitoring

## Contributing

1. Follow Go best practices and conventions
2. Add comprehensive tests for new features
3. Update documentation for any API changes
4. Ensure proper error handling and logging
5. Maintain JWT authentication patterns
6. Follow user ownership principles

## License

This project is licensed under the MIT License.
