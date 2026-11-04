-- Tenant Role Permissions SQL Script
-- Users Table
CREATE TABLE users (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    username VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    avatar VARCHAR(255),
    phone VARCHAR(20),
    verified TINYINT(1) DEFAULT 0,
    status TINYINT(1) DEFAULT 1,
    last_login DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Tenants Table
CREATE TABLE tenants (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    logo VARCHAR(255),
    description VARCHAR(255),
    status ENUM('active', 'suspended', 'deleted') DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Role Types Table
CREATE TABLE role_types (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Roles Table
CREATE TABLE roles (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    description VARCHAR(255),
    is_system_role TINYINT(1) DEFAULT 0,
    role_type_id BIGINT,
    tenant_id BIGINT DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (role_type_id) REFERENCES role_types(id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

-- Permissions Table
CREATE TABLE permissions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    description VARCHAR(255),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Services Table
CREATE TABLE services (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(255),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- Role Permissions Table
CREATE TABLE role_permissions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    role_id BIGINT,
    permission_id BIGINT,
    service_id BIGINT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (role_id) REFERENCES roles(id),
    FOREIGN KEY (permission_id) REFERENCES permissions(id),
    FOREIGN KEY (service_id) REFERENCES services(id)
);

-- Tenant User Roles Table
CREATE TABLE tenant_user_roles (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    tenant_id BIGINT,
    user_id BIGINT,
    role_id BIGINT,
    assigned_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    assigned_by_id BIGINT,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (role_id) REFERENCES roles(id),
    FOREIGN KEY (assigned_by_id) REFERENCES users(id)
);

-- Insert Tenant Mockup Data
-- Mockup Tenants
INSERT INTO tenants (name, logo, description, status) VALUES
('Tenant A', 'logo_a.png', 'Primary tenant', 'active'),
('Tenant B', 'logo_b.png', 'Secondary tenant', 'active');

-- Mockup Users
INSERT INTO users (email, username, password_hash, name, avatar, phone, verified, status) VALUES
('owner@tenant-a.com', 'owner_a', 'hashed_pass', 'Owner A', 'avatar_a.png', '1234567890', 1, 1),
('editor@tenant-a.com', 'editor_a', 'hashed_pass', 'Editor A', 'avatar_b.png', '1234567891', 1, 1),
('viewer@tenant-a.com', 'viewer_a', 'hashed_pass', 'Viewer A', 'avatar_c.png', '1234567892', 1, 1);

-- Insert Mockup Data
-- Role Types
INSERT INTO role_types (name) VALUES ('system'), ('custom');

-- System Roles
INSERT INTO roles (name, description, is_system_role, role_type_id) VALUES
('Owner', 'Full access to all resources', 1, 1),
('Editor', 'Edit access to most resources', 1, 1),
('Viewer', 'Read-only access', 1, 1);

-- Permissions
INSERT INTO permissions (name, description) VALUES
('View', 'Can view the resource'),
('Edit', 'Can edit the resource'),
('Delete', 'Can delete the resource');

-- Services
INSERT INTO services (name, description) VALUES
('Auth', 'User authentication service'),
('Storage', 'Cloud file storage'),
('Database', 'Database management service');

-- Custom Role Example
INSERT INTO roles (name, description, is_system_role, role_type_id, tenant_id) VALUES
('Custom Viewer', 'View only specific services', 0, 2, 1);

-- Role Permissions
INSERT INTO role_permissions (role_id, permission_id, service_id) VALUES
(4, 1, 1), -- Custom Viewer: View Auth
(4, 1, 2); -- Custom Viewer: View Storage

-- Procedure to Get User Info by Role
DELIMITER //
CREATE PROCEDURE GetUserInfoByRole(IN tenantId BIGINT, IN roleName VARCHAR(50))
BEGIN
    SELECT u.id, u.name, u.email, r.name AS role_name
    FROM users u
    JOIN tenant_user_roles tur ON u.id = tur.user_id
    JOIN roles r ON tur.role_id = r.id
    WHERE tur.tenant_id = tenantId AND r.name = roleName;
END //
DELIMITER ;
