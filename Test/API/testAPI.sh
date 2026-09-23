#!/bin/bash

API_URL="https://localhost:8100/module/api"
API_KEY="bigsecretrandomstring"  # reemplaza por tu clave real de módulo

# Ignorar errores de certificado autofirmado
CURL="curl -sk"

echo "=== 1. POST: Insert new user ==="
$CURL -X POST "$API_URL" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "table": "users",
    "columns": "name,surnames,role,password",
    "value": "TestUser,UserSurname,user,test1234"
  }'
echo -e "\n"

sleep 1

echo "=== 2. GET: Fetch inserted user ==="
$CURL -X GET "$API_URL" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "table": "users",
    "columns": "id,name,surnames,role",
    "condition": "name='\''TestUser'\''"
  }'
echo -e "\n"

sleep 1

echo "=== 3. PUT: Update user role ==="
$CURL -X PUT "$API_URL" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "table": "users",
    "columns": "role",
    "value": "admin",
    "condition": "name='\''TestUser'\''"
  }'
echo -e "\n"

sleep 1

echo "=== 4. GET: Verify updated role ==="
$CURL -X GET "$API_URL" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "table": "users",
    "columns": "id,name,surnames,role",
    "condition": "name='\''TestUser'\''"
  }'
echo -e "\n"

sleep 1

echo "=== 5. DELETE: Remove user ==="
$CURL -X DELETE "$API_URL" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "table": "users",
    "condition": "name='\''TestUser'\''"
  }'
echo -e "\n"

sleep 1

echo "=== 6. GET: Confirm user deleted ==="
$CURL -X GET "$API_URL" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "table": "users",
    "columns": "id,name,surnames,role",
    "condition": "name='\''TestUser'\''"
  }'
echo -e "\n"

echo "=== TEST COMPLETED ==="
