-- Create a schema for the project
CREATE SCHEMA IF NOT EXISTS nivek;

-- Create a sample table
CREATE TABLE IF NOT EXISTS nivek.users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert a sample user
INSERT INTO nivek.users (username, email) 
VALUES ('timallen', 'timallen@nivek.com') 
ON CONFLICT (username) DO NOTHING;