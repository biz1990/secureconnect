-- SecureConnect Cassandra Authentication Setup
-- This script sets up authentication for Cassandra
-- Execute this AFTER Cassandra is running and BEFORE schema initialization
-- Usage: cqlsh -u cassandra -p <password> < cassandra-auth-setup.cql

-- ==========================================
-- CREATE SECURECONNECT USER
-- ==========================================
-- Create a dedicated user for the application
CREATE USER IF NOT EXISTS 'secureconnect_user' WITH PASSWORD 'CHANGE_ME_SECURE_PASSWORD' NOSUPERUSER;

-- Grant all permissions on all keyspaces to the application user
GRANT ALL PERMISSIONS ON ALL KEYSPACES TO 'secureconnect_user';

-- ==========================================
-- CREATE KEYSPACE AS SUPERUSER
-- ==========================================
-- The keyspace creation needs to be done by the cassandra superuser
DROP KEYSPACE IF EXISTS secureconnect_ks;

CREATE KEYSPACE IF NOT EXISTS secureconnect_ks
  WITH replication = {
    'class': 'SimpleStrategy',
    'replication_factor': 1  -- For dev; use 3 for production
  }
  AND durable_writes = true;

-- Grant permissions on the keyspace
GRANT ALL PERMISSIONS ON KEYSPACE secureconnect_ks TO 'secureconnect_user';

-- ==========================================
-- VERIFICATION
-- ==========================================
-- List users to verify
LIST USERS;

-- List permissions
LIST ALL PERMISSIONS OF 'secureconnect_user';

-- Show keyspaces
DESCRIBE KEYSPACES;
