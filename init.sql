-- Create the status_pages table
CREATE TABLE IF NOT EXISTS status_pages (
    id SERIAL PRIMARY KEY,
    hostname TEXT UNIQUE NOT NULL,
    page_data_url TEXT NOT NULL
);

-- Insert test data
INSERT INTO status_pages (hostname, page_data_url) VALUES
    ('status.acme.com', 'http://backend:8080/acme'),
    ('status.example.com', 'http://backend:8080/example')
ON CONFLICT (hostname) DO NOTHING;
