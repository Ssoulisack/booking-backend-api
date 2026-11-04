-- Database Schema for Multi-Tenant Inventory Management SaaS
-- Version: 1.0
-- Created: 2023-11-15

SET FOREIGN_KEY_CHECKS = 0;
SET NAMES utf8mb4;
SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
SET time_zone = "+00:00";

-- --------------------------------------------------------
-- Database schema for multi-tenant inventory management
-- --------------------------------------------------------

--
-- Table structure for table `tenants`
--

CREATE TABLE IF NOT EXISTS `tenants` (
  `id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `business_type` varchar(100) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `status` enum('active','suspended','deleted') NOT NULL DEFAULT 'active',
  PRIMARY KEY (`id`),
  KEY `idx_tenant_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `users`
--

CREATE TABLE IF NOT EXISTS `users` (
  `id` char(36) NOT NULL,
  `email` varchar(255) NOT NULL,
  `password_hash` varchar(255) NOT NULL,
  `first_name` varchar(100) DEFAULT NULL,
  `last_name` varchar(100) DEFAULT NULL,
  `phone` varchar(20) DEFAULT NULL,
  `email_verified` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `last_login` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `email` (`email`),
  KEY `idx_user_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `tenant_users`
--

CREATE TABLE IF NOT EXISTS `tenant_users` (
  `tenant_id` char(36) NOT NULL,
  `user_id` char(36) NOT NULL,
  `is_owner` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`tenant_id`,`user_id`),
  KEY `user_id` (`user_id`),
  KEY `idx_tenant_user` (`tenant_id`,`user_id`),
  CONSTRAINT `tenant_users_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `tenant_users_ibfk_2` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `accounts`
--

CREATE TABLE IF NOT EXISTS `accounts` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `type` enum('warehouse','store','virtual') NOT NULL,
  `address` text DEFAULT NULL,
  `contact_phone` varchar(20) DEFAULT NULL,
  `is_active` tinyint(1) NOT NULL DEFAULT 1,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `tenant_id` (`tenant_id`),
  KEY `idx_account_tenant` (`tenant_id`),
  KEY `idx_account_active` (`tenant_id`,`is_active`),
  CONSTRAINT `accounts_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `roles`
--

CREATE TABLE IF NOT EXISTS `roles` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `name` varchar(50) NOT NULL,
  `description` text DEFAULT NULL,
  `is_system_role` tinyint(1) NOT NULL DEFAULT 0,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_role` (`tenant_id`,`name`),
  CONSTRAINT `roles_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `permissions`
--

CREATE TABLE IF NOT EXISTS `permissions` (
  `id` char(36) NOT NULL,
  `code` varchar(100) NOT NULL,
  `description` text DEFAULT NULL,
  `category` varchar(50) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `role_permissions`
--

CREATE TABLE IF NOT EXISTS `role_permissions` (
  `role_id` char(36) NOT NULL,
  `permission_id` char(36) NOT NULL,
  PRIMARY KEY (`role_id`,`permission_id`),
  KEY `permission_id` (`permission_id`),
  CONSTRAINT `role_permissions_ibfk_1` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `role_permissions_ibfk_2` FOREIGN KEY (`permission_id`) REFERENCES `permissions` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `user_account_roles`
--

CREATE TABLE IF NOT EXISTS `user_account_roles` (
  `user_id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `account_id` char(36) NOT NULL,
  `role_id` char(36) NOT NULL,
  `assigned_by` char(36) DEFAULT NULL,
  `assigned_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`,`tenant_id`,`account_id`,`role_id`),
  KEY `tenant_id` (`tenant_id`),
  KEY `account_id` (`account_id`),
  KEY `role_id` (`role_id`),
  KEY `assigned_by` (`assigned_by`),
  KEY `idx_user_tenant_account` (`user_id`,`tenant_id`,`account_id`),
  CONSTRAINT `user_account_roles_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE,
  CONSTRAINT `user_account_roles_ibfk_2` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `user_account_roles_ibfk_3` FOREIGN KEY (`account_id`) REFERENCES `accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `user_account_roles_ibfk_4` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `user_account_roles_ibfk_5` FOREIGN KEY (`assigned_by`) REFERENCES `users` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `product_categories`
--

CREATE TABLE IF NOT EXISTS `product_categories` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `name` varchar(255) NOT NULL,
  `parent_id` char(36) DEFAULT NULL,
  `description` text DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `tenant_id` (`tenant_id`),
  KEY `parent_id` (`parent_id`),
  KEY `idx_category_tenant` (`tenant_id`),
  KEY `idx_category_parent` (`tenant_id`,`parent_id`),
  CONSTRAINT `product_categories_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `product_categories_ibfk_2` FOREIGN KEY (`parent_id`) REFERENCES `product_categories` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `products`
--

CREATE TABLE IF NOT EXISTS `products` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `sku` varchar(100) NOT NULL,
  `name` varchar(255) NOT NULL,
  `description` text DEFAULT NULL,
  `category_id` char(36) DEFAULT NULL,
  `unit_type` varchar(50) NOT NULL,
  `barcode` varchar(100) DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_sku` (`tenant_id`,`sku`),
  KEY `category_id` (`category_id`),
  KEY `idx_product_tenant` (`tenant_id`),
  KEY `idx_product_category` (`tenant_id`,`category_id`),
  CONSTRAINT `products_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `products_ibfk_2` FOREIGN KEY (`category_id`) REFERENCES `product_categories` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `inventory`
--

CREATE TABLE IF NOT EXISTS `inventory` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `account_id` char(36) NOT NULL,
  `product_id` char(36) NOT NULL,
  `quantity` decimal(15,3) NOT NULL DEFAULT 0.000,
  `reorder_level` decimal(15,3) DEFAULT NULL,
  `last_updated` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_inventory_location` (`tenant_id`,`account_id`,`product_id`),
  KEY `account_id` (`account_id`),
  KEY `product_id` (`product_id`),
  KEY `idx_inventory_product` (`tenant_id`,`product_id`),
  KEY `idx_inventory_account` (`tenant_id`,`account_id`),
  CONSTRAINT `inventory_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `inventory_ibfk_2` FOREIGN KEY (`account_id`) REFERENCES `accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `inventory_ibfk_3` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `stock_movements`
--

CREATE TABLE IF NOT EXISTS `stock_movements` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `account_id` char(36) NOT NULL,
  `product_id` char(36) NOT NULL,
  `user_id` char(36) NOT NULL,
  `movement_type` enum('purchase','sale','transfer_in','transfer_out','adjustment','return') NOT NULL,
  `quantity` decimal(15,3) NOT NULL,
  `reference_id` varchar(100) DEFAULT NULL,
  `notes` text DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `account_id` (`account_id`),
  KEY `product_id` (`product_id`),
  KEY `user_id` (`user_id`),
  KEY `idx_movement_tenant` (`tenant_id`),
  KEY `idx_movement_account` (`tenant_id`,`account_id`),
  KEY `idx_movement_product` (`tenant_id`,`product_id`),
  KEY `idx_movement_date` (`tenant_id`,`created_at`),
  CONSTRAINT `stock_movements_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `stock_movements_ibfk_2` FOREIGN KEY (`account_id`) REFERENCES `accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `stock_movements_ibfk_3` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE CASCADE,
  CONSTRAINT `stock_movements_ibfk_4` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `stock_transfers`
--

CREATE TABLE IF NOT EXISTS `stock_transfers` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `from_account_id` char(36) NOT NULL,
  `to_account_id` char(36) NOT NULL,
  `user_id` char(36) NOT NULL,
  `status` enum('pending','in_transit','completed','cancelled') NOT NULL DEFAULT 'pending',
  `tracking_number` varchar(100) DEFAULT NULL,
  `notes` text DEFAULT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `completed_at` timestamp NULL DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `from_account_id` (`from_account_id`),
  KEY `to_account_id` (`to_account_id`),
  KEY `user_id` (`user_id`),
  KEY `idx_transfer_tenant` (`tenant_id`),
  KEY `idx_transfer_status` (`tenant_id`,`status`),
  CONSTRAINT `stock_transfers_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `stock_transfers_ibfk_2` FOREIGN KEY (`from_account_id`) REFERENCES `accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `stock_transfers_ibfk_3` FOREIGN KEY (`to_account_id`) REFERENCES `accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `stock_transfers_ibfk_4` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `stock_transfer_items`
--

CREATE TABLE IF NOT EXISTS `stock_transfer_items` (
  `id` char(36) NOT NULL,
  `transfer_id` char(36) NOT NULL,
  `product_id` char(36) NOT NULL,
  `quantity` decimal(15,3) NOT NULL,
  `received_quantity` decimal(15,3) DEFAULT NULL,
  `status` enum('pending','partial','complete') NOT NULL DEFAULT 'pending',
  PRIMARY KEY (`id`),
  KEY `transfer_id` (`transfer_id`),
  KEY `product_id` (`product_id`),
  KEY `idx_transfer_item` (`transfer_id`),
  CONSTRAINT `stock_transfer_items_ibfk_1` FOREIGN KEY (`transfer_id`) REFERENCES `stock_transfers` (`id`) ON DELETE CASCADE,
  CONSTRAINT `stock_transfer_items_ibfk_2` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Table structure for table `invitations`
--

CREATE TABLE IF NOT EXISTS `invitations` (
  `id` char(36) NOT NULL,
  `tenant_id` char(36) NOT NULL,
  `email` varchar(255) NOT NULL,
  `role_id` char(36) NOT NULL,
  `account_id` char(36) DEFAULT NULL,
  `token` varchar(255) NOT NULL,
  `invited_by` char(36) NOT NULL,
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `expires_at` timestamp NOT NULL,
  `status` enum('pending','accepted','expired','revoked') NOT NULL DEFAULT 'pending',
  PRIMARY KEY (`id`),
  KEY `role_id` (`role_id`),
  KEY `account_id` (`account_id`),
  KEY `invited_by` (`invited_by`),
  KEY `idx_invitation_tenant` (`tenant_id`),
  KEY `idx_invitation_email` (`email`),
  KEY `idx_invitation_token` (`token`),
  KEY `idx_invitation_status` (`status`),
  CONSTRAINT `invitations_ibfk_1` FOREIGN KEY (`tenant_id`) REFERENCES `tenants` (`id`) ON DELETE CASCADE,
  CONSTRAINT `invitations_ibfk_2` FOREIGN KEY (`role_id`) REFERENCES `roles` (`id`) ON DELETE CASCADE,
  CONSTRAINT `invitations_ibfk_3` FOREIGN KEY (`account_id`) REFERENCES `accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `invitations_ibfk_4` FOREIGN KEY (`invited_by`) REFERENCES `users` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

--
-- Insert default permissions
--

INSERT IGNORE INTO `permissions` (`id`, `code`, `description`, `category`) VALUES
(UUID(), 'tenant:manage', 'Manage tenant settings', 'Tenant'),
(UUID(), 'user:invite', 'Invite new users', 'User'),
(UUID(), 'user:manage', 'Manage users', 'User'),
(UUID(), 'role:manage', 'Manage roles', 'User'),
(UUID(), 'account:create', 'Create accounts', 'Account'),
(UUID(), 'account:manage', 'Manage accounts', 'Account'),
(UUID(), 'product:create', 'Create products', 'Product'),
(UUID(), 'product:manage', 'Manage products', 'Product'),
(UUID(), 'inventory:view', 'View inventory', 'Inventory'),
(UUID(), 'inventory:manage', 'Manage inventory', 'Inventory'),
(UUID(), 'report:view', 'View reports', 'Report');

SET FOREIGN_KEY_CHECKS = 1;