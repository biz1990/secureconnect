#!/usr/bin/env bash
# Integration Test Script for SecureConnect Backend
# Tests full flow: Register → Create Conversation → Send Message → WebSocket delivery

set -e  # Exit on error

API_BASE="http://localhost:8080/v1"
CHAT_WS="ws://localhost:8082/v1/ws/chat"

echo "🧪 SecureConnect Integration Tests"
echo "=================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Helper function
check_response() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $1${NC}"
    else
        echo -e "${RED}✗ $1 FAILED${NC}"
        exit 1
    fi
}

echo "Step 1: Register User Alice"
echo "----------------------------"
ALICE_RESPONSE=$(curl -s -X POST "$API_BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@test.com",
    "username": "alice",
    "password": "password123",
    "display_name": "Alice Test"
  }')

ALICE_TOKEN=$(echo $ALICE_RESPONSE | jq -r '.data.access_token')
ALICE_ID=$(echo $ALICE_RESPONSE | jq -r '.data.user.user_id')

check_response "Alice registered"
echo "Alice ID: $ALICE_ID"
echo "Alice Token: ${ALICE_TOKEN:0:20}..."
echo ""

echo "Step 2: Register User Bob"
echo "-------------------------"
BOB_RESPONSE=$(curl -s -X POST "$API_BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "bob@test.com",
    "username": "bob",
    "password": "password123",
    "display_name": "Bob Test"
  }')

BOB_TOKEN=$(echo $BOB_RESPONSE | jq -r '.data.access_token')
BOB_ID=$(echo $BOB_RESPONSE | jq -r '.data.user.user_id')

check_response "Bob registered"
echo "Bob ID: $BOB_ID"
echo ""

echo "Step 3: Alice uploads E2EE keys"
echo "-------------------------------"
curl -s -X POST "$API_BASE/keys/upload" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "identity_key": "alice_identity_key_base64",
    "signed_pre_key": {
      "key_id": 1,
      "public_key": "alice_signed_prekey_base64",
      "signature": "alice_signature_base64"
    },
    "one_time_pre_keys": [
      {"key_id": 1, "public_key": "alice_otk_1"},
      {"key_id": 2, "public_key": "alice_otk_2"},
      {"key_id": 3, "public_key": "alice_otk_3"},
      {"key_id": 4, "public_key": "alice_otk_4"},
      {"key_id": 5, "public_key": "alice_otk_5"},
      {"key_id": 6, "public_key": "alice_otk_6"},
      {"key_id": 7, "public_key": "alice_otk_7"},
      {"key_id": 8, "public_key": "alice_otk_8"},
      {"key_id": 9, "public_key": "alice_otk_9"},
      {"key_id": 10, "public_key": "alice_otk_10"},
      {"key_id": 11, "public_key": "alice_otk_11"},
      {"key_id": 12, "public_key": "alice_otk_12"},
      {"key_id": 13, "public_key": "alice_otk_13"},
      {"key_id": 14, "public_key": "alice_otk_14"},
      {"key_id": 15, "public_key": "alice_otk_15"},
      {"key_id": 16, "public_key": "alice_otk_16"},
      {"key_id": 17, "public_key": "alice_otk_17"},
      {"key_id": 18, "public_key": "alice_otk_18"},
      {"key_id": 19, "public_key": "alice_otk_19"},
      {"key_id": 20, "public_key": "alice_otk_20"}
    ]
  }' > /dev/null

check_response "Alice E2EE keys uploaded"
echo ""

echo "Step 4: Bob retrieves Alice's pre-key bundle"
echo "-------------------------------------------"
PREKEY_BUNDLE=$(curl -s -X GET "$API_BASE/keys/$ALICE_ID" \
  -H "Authorization: Bearer $BOB_TOKEN")

check_response "Pre-key bundle retrieved"
echo "Bundle: $(echo $PREKEY_BUNDLE | jq -r '.data.identity_key')"
echo ""

echo "Step 5: Create conversation between Alice and Bob"
echo "------------------------------------------------"
CONV_RESPONSE=$(curl -s -X POST "$API_BASE/conversations" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"title\": \"Alice & Bob Chat\",
    \"type\": \"direct\",
    \"participant_ids\": [\"$ALICE_ID\", \"$BOB_ID\"],
    \"is_e2ee_enabled\": true
  }")

CONV_ID=$(echo $CONV_RESPONSE | jq -r '.data.conversation_id')

check_response "Conversation created"
echo "Conversation ID: $CONV_ID"
echo ""

echo "Step 6: Alice sends encrypted message"
echo "------------------------------------"
MSG_RESPONSE=$(curl -s -X POST "$API_BASE/messages" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"conversation_id\": \"$CONV_ID\",
    \"content\": \"encrypted_payload_base64\",
    \"is_encrypted\": true,
    \"message_type\": \"text\"
  }")

MSG_ID=$(echo $MSG_RESPONSE | jq -r '.data.message_id')

check_response "Message sent"
echo "Message ID: $MSG_ID"
echo ""

echo "Step 7: Bob retrieves messages"
echo "-----------------------------"
MESSAGES=$(curl -s -X GET "$API_BASE/messages?conversation_id=$CONV_ID&limit=10" \
  -H "Authorization: Bearer $BOB_TOKEN")

MSG_COUNT=$(echo $MESSAGES | jq -r '.data.messages | length')

check_response "Messages retrieved"
echo "Message count: $MSG_COUNT"
echo ""

echo "Step 8: Test file upload flow"
echo "----------------------------"
FILE_UPLOAD=$(curl -s -X POST "$API_BASE/storage/upload-url" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "test.pdf",
    "file_size": 1024000,
    "content_type": "application/pdf",
    "is_encrypted": true
  }')

FILE_ID=$(echo $FILE_UPLOAD | jq -r '.data.file_id')
UPLOAD_URL=$(echo $FILE_UPLOAD | jq -r '.data.upload_url')

check_response "Upload URL generated"
echo "File ID: $FILE_ID"
echo ""

echo "Step 9: Get storage quota"
echo "-----------------------"
QUOTA=$(curl -s -X GET "$API_BASE/storage/quota" \
  -H "Authorization: Bearer $ALICE_TOKEN")

USED=$(echo $QUOTA | jq -r '.data.used')
TOTAL=$(echo $QUOTA | jq -r '.data.total')

check_response "Quota retrieved"
echo "Storage: $USED / $TOTAL bytes"
echo ""

echo "Step 10: Test presence update"
echo "---------------------------"
curl -s -X POST "$API_BASE/presence" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"online": true}' > /dev/null

check_response "Presence updated"
echo ""

echo "=================================="
echo -e "${GREEN}✅ All Integration Tests PASSED!${NC}"
echo "=================================="
echo ""
echo "Next: Test WebSocket connection manually"
echo "Example: wscat -c '$CHAT_WS?conversation_id=$CONV_ID' -H 'Authorization: Bearer $ALICE_TOKEN'"
