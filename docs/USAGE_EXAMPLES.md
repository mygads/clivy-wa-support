# Campaign API Usage Examples

## JavaScript/Node.js Examples

### Setup
```javascript
const axios = require('axios');

const API_BASE_URL = 'https://your-api-domain.com';
const JWT_TOKEN = 'your-jwt-token-here';
const WHATSAPP_SESSION_TOKEN = 'your-whatsapp-session-token'; // Required for contact sync

const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Authorization': `Bearer ${JWT_TOKEN}`,
    'Content-Type': 'application/json'
  }
});
```

### 1. Contact Management

#### Sync Contacts
```javascript
async function syncContacts() {
  try {
    // Contact sync requires both JWT and WhatsApp session token
    const response = await apiClient.post('/bulk/contact/sync', {}, {
      headers: {
        'token': WHATSAPP_SESSION_TOKEN // Additional WhatsApp session token
      }
    });
    console.log('Sync Result:', response.data);
    return response.data;
  } catch (error) {
    console.error('Sync Error:', error.response?.data || error.message);
    throw error;
  }
}
```

#### Get Contact List
```javascript
async function getContacts() {
  try {
    const response = await apiClient.get('/bulk/contact');
    console.log('Contacts:', response.data.data);
    return response.data.data;
  } catch (error) {
    console.error('Get Contacts Error:', error.response?.data || error.message);
    throw error;
  }
}
```

#### Add Contacts Manually
```javascript
async function addContacts(contacts) {
  try {
    const response = await apiClient.post('/bulk/contact/add', {
      contacts: contacts
    });
    console.log('Add Contacts Result:', response.data);
    return response.data;
  } catch (error) {
    console.error('Add Contacts Error:', error.response?.data || error.message);
    throw error;
  }
}

// Usage
const newContacts = [
  { phone: '628123456789', full_name: 'John Doe' },
  { phone: '628987654321', full_name: 'Jane Smith' }
];
addContacts(newContacts);
```

### 2. Campaign Template Management

#### Create Text Campaign Template
```javascript
async function createTextCampaign(name, message) {
  try {
    const response = await apiClient.post('/bulk/campaign', {
      name: name,
      type: 'text',
      message_body: message
    });
    console.log('Campaign Created:', response.data);
    return response.data.data;
  } catch (error) {
    console.error('Create Campaign Error:', error.response?.data || error.message);
    throw error;
  }
}

// Usage
createTextCampaign('Welcome Message', 'Selamat datang di layanan kami!');
```

#### Create Image Campaign Template
```javascript
async function createImageCampaign(name, message, imageUrl, caption) {
  try {
    const response = await apiClient.post('/bulk/campaign', {
      name: name,
      type: 'image',
      message_body: message,
      image_url: imageUrl,
      caption: caption
    });
    console.log('Image Campaign Created:', response.data);
    return response.data.data;
  } catch (error) {
    console.error('Create Image Campaign Error:', error.response?.data || error.message);
    throw error;
  }
}

// Usage
createImageCampaign(
  'Product Promo',
  'Promo spesial bulan ini!',
  'https://example.com/promo.jpg',
  'Jangan lewatkan kesempatan emas ini!'
);
```

#### Get All Campaign Templates
```javascript
async function getCampaigns() {
  try {
    const response = await apiClient.get('/bulk/campaign');
    console.log('Campaigns:', response.data.data);
    return response.data.data;
  } catch (error) {
    console.error('Get Campaigns Error:', error.response?.data || error.message);
    throw error;
  }
}
```

#### Update Campaign Template
```javascript
async function updateCampaign(campaignId, updates) {
  try {
    const response = await apiClient.put(`/bulk/campaign/${campaignId}`, updates);
    console.log('Campaign Updated:', response.data);
    return response.data.data;
  } catch (error) {
    console.error('Update Campaign Error:', error.response?.data || error.message);
    throw error;
  }
}

// Usage
updateCampaign(1, {
  name: 'Updated Welcome Message',
  message_body: 'Selamat datang di layanan terbaru kami!'
});
```

### 3. Bulk Campaign Execution

#### Execute Immediate Bulk Campaign
```javascript
async function executeBulkCampaign(campaignId, name, phoneNumbers) {
  try {
    const response = await apiClient.post('/bulk/campaign/execute', {
      campaign_id: campaignId,
      name: name,
      phone: phoneNumbers,
      send_sync: 'now'
    });
    console.log('Bulk Campaign Executed:', response.data);
    return response.data.data;
  } catch (error) {
    console.error('Execute Bulk Campaign Error:', error.response?.data || error.message);
    throw error;
  }
}

// Usage
const phoneNumbers = ['628123456789', '628987654321', '628555666777'];
executeBulkCampaign(1, 'Welcome Batch 1', phoneNumbers);
```

#### Schedule Bulk Campaign
```javascript
async function scheduleBulkCampaign(campaignId, name, phoneNumbers, scheduleDateTime) {
  try {
    const response = await apiClient.post('/bulk/campaign/execute', {
      campaign_id: campaignId,
      name: name,
      phone: phoneNumbers,
      send_sync: scheduleDateTime
    });
    console.log('Bulk Campaign Scheduled:', response.data);
    return response.data.data;
  } catch (error) {
    console.error('Schedule Bulk Campaign Error:', error.response?.data || error.message);
    throw error;
  }
}

// Usage - Schedule for tomorrow 9 AM
const tomorrow9AM = new Date();
tomorrow9AM.setDate(tomorrow9AM.getDate() + 1);
tomorrow9AM.setHours(9, 0, 0, 0);
const scheduleTime = tomorrow9AM.toISOString().slice(0, 19).replace('T', ' ');

scheduleBulkCampaign(1, 'Welcome Batch 2', phoneNumbers, scheduleTime);
```

### 4. Bulk Campaign Monitoring

#### Get All Bulk Campaigns
```javascript
async function getBulkCampaigns() {
  try {
    const response = await apiClient.get('/bulk/campaigns');
    console.log('Bulk Campaigns:', response.data.data);
    return response.data.data;
  } catch (error) {
    console.error('Get Bulk Campaigns Error:', error.response?.data || error.message);
    throw error;
  }
}
```

#### Get Bulk Campaign Detail with Items
```javascript
async function getBulkCampaignDetail(bulkCampaignId) {
  try {
    const response = await apiClient.get(`/bulk/campaigns/${bulkCampaignId}`);
    console.log('Bulk Campaign Detail:', response.data.data);
    return response.data.data;
  } catch (error) {
    console.error('Get Bulk Campaign Detail Error:', error.response?.data || error.message);
    throw error;
  }
}
```

### 5. Complete Workflow Example

```javascript
async function completeWorkflow() {
  try {
    console.log('=== Starting Complete Workflow ===');

    // 1. Sync contacts
    console.log('\n1. Syncing contacts...');
    await syncContacts();

    // 2. Get contacts to verify
    console.log('\n2. Getting contacts...');
    const contacts = await getContacts();
    console.log(`Found ${contacts.length} contacts`);

    // 3. Create campaign template
    console.log('\n3. Creating campaign template...');
    const campaign = await createTextCampaign(
      'Welcome New Users',
      'Selamat datang! Terima kasih telah bergabung dengan layanan kami.'
    );

    // 4. Execute bulk campaign
    console.log('\n4. Executing bulk campaign...');
    const phoneNumbers = contacts.slice(0, 5).map(contact => contact.phone); // Take first 5
    const bulkCampaign = await executeBulkCampaign(
      campaign.id,
      'Welcome Campaign Batch 1',
      phoneNumbers
    );

    // 5. Monitor progress
    console.log('\n5. Monitoring campaign progress...');
    
    // Wait a bit then check status
    setTimeout(async () => {
      const detail = await getBulkCampaignDetail(bulkCampaign.bulk_campaign_id);
      console.log(`Campaign Status: ${detail.bulk_campaign.status}`);
      console.log(`Sent: ${detail.bulk_campaign.sent_count}/${detail.bulk_campaign.total_count}`);
      
      // Show individual item status
      detail.items.forEach(item => {
        console.log(`${item.phone}: ${item.status}`);
      });
    }, 5000); // Check after 5 seconds

    console.log('\n=== Workflow Completed ===');

  } catch (error) {
    console.error('Workflow Error:', error);
  }
}

// Run the complete workflow
completeWorkflow();
```

### 6. Utility Functions

#### Format Phone Number
```javascript
function formatPhoneNumber(phone) {
  // Remove all non-digit characters
  let cleaned = phone.replace(/\D/g, '');
  
  // Add 62 prefix if starts with 0
  if (cleaned.startsWith('0')) {
    cleaned = '62' + cleaned.substring(1);
  }
  
  // Add 62 prefix if doesn't start with 62
  if (!cleaned.startsWith('62')) {
    cleaned = '62' + cleaned;
  }
  
  return cleaned;
}

// Usage
const formattedPhone = formatPhoneNumber('0812-3456-789'); // Returns: 6281234567890
```

#### Batch Process Phone Numbers
```javascript
async function batchExecuteCampaign(campaignId, allPhoneNumbers, batchSize = 50) {
  const batches = [];
  
  // Split phone numbers into batches
  for (let i = 0; i < allPhoneNumbers.length; i += batchSize) {
    batches.push(allPhoneNumbers.slice(i, i + batchSize));
  }
  
  const results = [];
  
  // Execute each batch
  for (let i = 0; i < batches.length; i++) {
    const batch = batches[i];
    console.log(`Executing batch ${i + 1}/${batches.length} (${batch.length} numbers)`);
    
    try {
      const result = await executeBulkCampaign(
        campaignId,
        `Campaign Batch ${i + 1}`,
        batch
      );
      results.push(result);
      
      // Wait between batches to avoid rate limiting
      if (i < batches.length - 1) {
        await new Promise(resolve => setTimeout(resolve, 2000));
      }
    } catch (error) {
      console.error(`Batch ${i + 1} failed:`, error.message);
    }
  }
  
  return results;
}
```

#### Monitor Campaign Progress
```javascript
async function monitorCampaignProgress(bulkCampaignId, intervalSeconds = 10) {
  const startTime = Date.now();
  
  const checkProgress = async () => {
    try {
      const detail = await getBulkCampaignDetail(bulkCampaignId);
      const campaign = detail.bulk_campaign;
      
      console.log(`[${new Date().toLocaleTimeString()}] Status: ${campaign.status}`);
      console.log(`Progress: ${campaign.sent_count}/${campaign.total_count} sent, ${campaign.failed_count} failed`);
      
      if (campaign.status === 'completed' || campaign.status === 'failed') {
        const duration = Math.round((Date.now() - startTime) / 1000);
        console.log(`Campaign finished in ${duration} seconds`);
        
        // Show failed items if any
        const failedItems = detail.items.filter(item => item.status === 'failed');
        if (failedItems.length > 0) {
          console.log('Failed items:');
          failedItems.forEach(item => {
            console.log(`- ${item.phone}: ${item.error_message}`);
          });
        }
        
        return;
      }
      
      // Continue monitoring
      setTimeout(checkProgress, intervalSeconds * 1000);
      
    } catch (error) {
      console.error('Monitor error:', error.message);
      setTimeout(checkProgress, intervalSeconds * 1000);
    }
  };
  
  checkProgress();
}

// Usage
monitorCampaignProgress(1, 5); // Check every 5 seconds
```

## Python Examples

### Setup
```python
import requests
import time
from datetime import datetime, timedelta

API_BASE_URL = 'https://your-api-domain.com'
JWT_TOKEN = 'your-jwt-token-here'
WHATSAPP_SESSION_TOKEN = 'your-whatsapp-session-token'  # Required for contact sync

headers = {
    'Authorization': f'Bearer {JWT_TOKEN}',
    'Content-Type': 'application/json'
}
```

### Sync and Execute Campaign
```python
def sync_and_execute_campaign():
    # Sync contacts (requires both JWT and WhatsApp session token)
    sync_headers = headers.copy()
    sync_headers['token'] = WHATSAPP_SESSION_TOKEN
    
    response = requests.post(f'{API_BASE_URL}/bulk/contact/sync', headers=sync_headers)
    print('Sync result:', response.json())
    
    # Get contacts
    response = requests.get(f'{API_BASE_URL}/bulk/contact', headers=headers)
    contacts = response.json()['data']
    print(f'Found {len(contacts)} contacts')
    
    # Create campaign
    campaign_data = {
        'name': 'Python Campaign',
        'type': 'text',
        'message_body': 'Hello from Python!'
    }
    response = requests.post(f'{API_BASE_URL}/bulk/campaign', headers=headers, json=campaign_data)
    campaign = response.json()['data']
    
    # Execute bulk campaign
    phone_numbers = [contact['phone'] for contact in contacts[:3]]
    bulk_data = {
        'campaign_id': campaign['id'],
        'name': 'Python Bulk Test',
        'phone': phone_numbers,
        'send_sync': 'now'
    }
    response = requests.post(f'{API_BASE_URL}/bulk/campaign/execute', headers=headers, json=bulk_data)
    print('Bulk campaign result:', response.json())

sync_and_execute_campaign()
```

## Error Handling Best Practices

```javascript
class CampaignAPIClient {
  constructor(baseURL, jwtToken) {
    this.client = axios.create({
      baseURL: baseURL,
      headers: {
        'Authorization': `Bearer ${jwtToken}`,
        'Content-Type': 'application/json'
      }
    });
    
    // Add response interceptor for error handling
    this.client.interceptors.response.use(
      response => response,
      error => {
        if (error.response) {
          // Server responded with error status
          const { status, data } = error.response;
          console.error(`API Error ${status}:`, data.message || data);
          
          if (status === 401) {
            // Handle unauthorized - maybe refresh token
            console.error('Unauthorized - check JWT token');
          }
        } else {
          // Network error
          console.error('Network Error:', error.message);
        }
        return Promise.reject(error);
      }
    );
  }
  
  async syncContacts() {
    try {
      const response = await this.client.post('/bulk/contact/sync');
      return response.data;
    } catch (error) {
      throw new Error(`Failed to sync contacts: ${error.response?.data?.message || error.message}`);
    }
  }
  
  // Add other methods...
}

// Usage
const client = new CampaignAPIClient(API_BASE_URL, JWT_TOKEN);
client.syncContacts()
  .then(result => console.log('Success:', result))
  .catch(error => console.error('Error:', error.message));
```
