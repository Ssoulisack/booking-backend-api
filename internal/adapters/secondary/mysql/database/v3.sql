-- Create the database
CREATE DATABASE IF NOT EXISTS saas_user_management;
USE saas_user_management;

-- Tenants table
CREATE TABLE tenants (
    tenant_id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Changed from 'products' to 'services' table
CREATE TABLE services (
    service_id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Roles table (Owner, Editor, Viewer, etc.)
CREATE TABLE roles (
    role_id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    description TEXT,
    is_system_role BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Permissions table
CREATE TABLE permissions (
    permission_id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Role-Permission mapping
CREATE TABLE role_permissions (
    role_id VARCHAR(36),
    permission_id VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (role_id, permission_id),
    FOREIGN KEY (role_id) REFERENCES roles(role_id),
    FOREIGN KEY (permission_id) REFERENCES permissions(permission_id)
);

-- Users table
CREATE TABLE users (
    user_id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Tenant Users (junction table for multi-tenancy)
CREATE TABLE tenant_users (
    tenant_user_id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36),
    user_id VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id),
    FOREIGN KEY (user_id) REFERENCES users(user_id),
    UNIQUE KEY (tenant_id, user_id)
);

-- Tenant User Roles (roles specific to tenant)
CREATE TABLE tenant_user_roles (
    tenant_user_role_id VARCHAR(36) PRIMARY KEY,
    tenant_user_id VARCHAR(36),
    role_id VARCHAR(36),
    assigned_by VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_user_id) REFERENCES tenant_users(tenant_user_id),
    FOREIGN KEY (role_id) REFERENCES roles(role_id),
    FOREIGN KEY (assigned_by) REFERENCES users(user_id)
);

-- Changed from 'tenant_product_access' to 'tenant_service_access'
CREATE TABLE tenant_service_access (
    access_id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36),
    service_id VARCHAR(36),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(tenant_id),
    FOREIGN KEY (service_id) REFERENCES services(service_id),
    UNIQUE KEY (tenant_id, service_id)
);

-- Insert mock data for Tenants
INSERT INTO tenants (tenant_id, name) VALUES
('tnt_001', 'MLV Corporation'),
('tnt_002', 'Acme Inc.'),
('tnt_003', 'Tech Solutions');

-- Insert mock data for Services (previously Products)
INSERT INTO services (service_id, name, description) VALUES
('srv_001', 'Analytics', 'User analytics and tracking'),
('srv_002', 'Database', 'Cloud database services'),
('srv_003', 'Authentication', 'User authentication system'),
('srv_004', 'Storage', 'File storage services'),
('srv_005', 'Messaging', 'Push notifications and messaging'),
('srv_006', 'Crashlytics', 'Crash reporting tool'),
('srv_007', 'Performance', 'Performance monitoring'),
('srv_008', 'Remote Config', 'Remote configuration management');

-- [Rest of the inserts remain the same, just changing product references to service]
-- Insert mock data for Roles
INSERT INTO roles (role_id, name, description, is_system_role) VALUES
('role_001', 'Owner', 'Full access to all tenant resources', TRUE),
('role_002', 'Editor', 'Can create and edit content', TRUE),
('role_003', 'Viewer', 'Can view content only', TRUE),
('role_004', 'Developer Admin', 'Admin access to development tools', FALSE),
('role_005', 'Quality Admin', 'Admin access to quality tools', FALSE),
('role_006', 'Growth Admin', 'Admin access to growth tools', FALSE),
('role_007', 'Billing Admin', 'Can manage billing and payments', FALSE);

-- Insert mock data for Permissions
INSERT INTO permissions (permission_id, name, description) VALUES
('perm_001', 'tenant.manage', 'Manage tenant settings'),
('perm_002', 'user.invite', 'Invite new users'),
('perm_003', 'user.manage', 'Manage user roles and permissions'),
('perm_004', 'content.create', 'Create new content'),
('perm_005', 'content.read', 'View content'),
('perm_006', 'content.update', 'Edit existing content'),
('perm_007', 'content.delete', 'Delete content'),
('perm_008', 'analytics.view', 'View analytics data'),
('perm_009', 'database.manage', 'Manage database settings'),
('perm_010', 'auth.manage', 'Manage authentication settings'),
('perm_011', 'storage.manage', 'Manage file storage'),
('perm_012', 'messaging.send', 'Send messages/notifications'),
('perm_013', 'crash.reports.view', 'View crash reports'),
('perm_014', 'performance.view', 'View performance metrics'),
('perm_015', 'config.manage', 'Manage remote configuration'),
('perm_016', 'billing.manage', 'Manage billing information');

-- Assign permissions to roles
INSERT INTO role_permissions (role_id, permission_id) VALUES
-- Owner gets all permissions
('role_001', 'perm_001'), ('role_001', 'perm_002'), ('role_001', 'perm_003'),
('role_001', 'perm_004'), ('role_001', 'perm_005'), ('role_001', 'perm_006'),
('role_001', 'perm_007'), ('role_001', 'perm_008'), ('role_001', 'perm_009'),
('role_001', 'perm_010'), ('role_001', 'perm_011'), ('role_001', 'perm_012'),
('role_001', 'perm_013'), ('role_001', 'perm_014'), ('role_001', 'perm_015'),
('role_001', 'perm_016'),

-- Editor permissions
('role_002', 'perm_004'), ('role_002', 'perm_005'), ('role_002', 'perm_006'),
('role_002', 'perm_008'),

-- Viewer permissions
('role_003', 'perm_005'), ('role_003', 'perm_008'),

-- Developer Admin permissions
('role_004', 'perm_004'), ('role_004', 'perm_005'), ('role_004', 'perm_006'),
('role_004', 'perm_007'), ('role_004', 'perm_008'), ('role_004', 'perm_009'),
('role_004', 'perm_010'), ('role_004', 'perm_011'),

-- Quality Admin permissions
('role_005', 'perm_005'), ('role_005', 'perm_008'), ('role_005', 'perm_013'),
('role_005', 'perm_014'),

-- Growth Admin permissions
('role_006', 'perm_005'), ('role_006', 'perm_008'), ('role_006', 'perm_012'),
('role_006', 'perm_015');

-- Insert mock users
INSERT INTO users (user_id, email, first_name, last_name) VALUES
('user_001', 'thongphetmlv@gmail.com', 'Thongphet', 'MLV'),
('user_002', 'sounymlv@gmail.com', 'Souny', 'MLV'),
('user_003', 'admin@acme.com', 'John', 'Doe'),
('user_004', 'dev@techsolutions.com', 'Jane', 'Smith'),
('user_005', 'viewer@mlv.com', 'Bob', 'Johnson');

-- Assign users to tenants
INSERT INTO tenant_users (tenant_user_id, tenant_id, user_id) VALUES
('tusr_001', 'tnt_001', 'user_001'),
('tusr_002', 'tnt_001', 'user_002'),
('tusr_003', 'tnt_002', 'user_003'),
('tusr_004', 'tnt_003', 'user_004'),
('tusr_005', 'tnt_001', 'user_005');

-- Assign roles to tenant users (similar to your screenshot)
INSERT INTO tenant_user_roles (tenant_user_role_id, tenant_user_id, role_id, assigned_by) VALUES
-- MLV Corporation users
('tur_001', 'tusr_001', 'role_001', 'user_001'),  -- Owner
('tur_002', 'tusr_002', 'role_004', 'user_001'),  -- Developer Admin
('tur_003', 'tusr_005', 'role_003', 'user_001'),  -- Viewer

-- Acme Inc. user
('tur_004', 'tusr_003', 'role_001', 'user_003'),  -- Owner

-- Tech Solutions user
('tur_005', 'tusr_004', 'role_002', 'user_004');  -- Editor

-- Enable services for tenants (previously products)
INSERT INTO tenant_service_access (access_id, tenant_id, service_id) VALUES
-- MLV Corporation has all services
('acc_001', 'tnt_001', 'srv_001'),
('acc_002', 'tnt_001', 'srv_002'),
('acc_003', 'tnt_001', 'srv_003'),
('acc_004', 'tnt_001', 'srv_004'),
('acc_005', 'tnt_001', 'srv_005'),
('acc_006', 'tnt_001', 'srv_006'),
('acc_007', 'tnt_001', 'srv_007'),
('acc_008', 'tnt_001', 'srv_008'),

-- Acme Inc. has basic services
('acc_009', 'tnt_002', 'srv_001'),
('acc_010', 'tnt_002', 'srv_002'),
('acc_011', 'tnt_002', 'srv_003'),

-- Tech Solutions has developer-focused services
('acc_012', 'tnt_003', 'srv_001'),
('acc_013', 'tnt_003', 'srv_002'),
('acc_014', 'tnt_003', 'srv_003'),
('acc_015', 'tnt_003', 'srv_004'),
('acc_016', 'tnt_003', 'srv_006'),
('acc_017', 'tnt_003', 'srv_007');