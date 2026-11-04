-- Location tables
CREATE TABLE locations (
    location_id INT AUTO_INCREMENT PRIMARY KEY,
    location_name VARCHAR(100) NOT NULL,
    location_type ENUM('warehouse', 'branch') NOT NULL,
    address VARCHAR(255) NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100) NOT NULL,
    postal_code VARCHAR(20) NOT NULL,
    country VARCHAR(100) NOT NULL,
    phone VARCHAR(20) NOT NULL,
    email VARCHAR(100),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_location_type (location_type),
    INDEX idx_location_active (is_active)
);

CREATE TABLE shelves (
    shelf_id INT AUTO_INCREMENT PRIMARY KEY,
    location_id INT NOT NULL,
    shelf_code VARCHAR(50) NOT NULL UNIQUE,
    shelf_type ENUM('pallet', 'bin', 'rack') NOT NULL,
    capacity INT NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (location_id) REFERENCES locations(location_id) ON DELETE CASCADE,
    INDEX idx_shelf_location (location_id),
    INDEX idx_shelf_active (is_active)
);
-- Product categories
CREATE TABLE product_categories (
    category_id INT AUTO_INCREMENT PRIMARY KEY,
    category_name VARCHAR(50) NOT NULL,
    category_type ENUM('perishable', 'non-perishable', 'frozen', 'dry goods') NOT NULL,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_category_name (category_name)
);

-- Products
CREATE TABLE products (
    product_id INT AUTO_INCREMENT PRIMARY KEY,
    product_code VARCHAR(50) NOT NULL,
    product_name VARCHAR(100) NOT NULL,
    description TEXT,
    category_id INT NOT NULL,
    unit_of_measure VARCHAR(20) NOT NULL,
    purchase_unit VARCHAR(20) NOT NULL,
    purchase_unit_conversion DECIMAL(10,3) NOT NULL COMMENT 'Conversion factor from purchase unit to usage unit (e.g., 12 if purchasing by dozen and using individually)',
    min_stock_level DECIMAL(10,3) NOT NULL,
    max_stock_level DECIMAL(10,3) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (category_id) REFERENCES product_categories(category_id),
    UNIQUE KEY uk_product_code (product_code),
    INDEX idx_product_category (category_id),
    INDEX idx_product_active (is_active)
);

-- Vendors
CREATE TABLE vendors (
    vendor_id INT AUTO_INCREMENT PRIMARY KEY,
    vendor_name VARCHAR(100) NOT NULL,
    contact_person VARCHAR(100),
    phone VARCHAR(20) NOT NULL,
    email VARCHAR(100),
    address VARCHAR(255),
    city VARCHAR(100),
    state VARCHAR(100),
    postal_code VARCHAR(20),
    country VARCHAR(100),
    tax_id VARCHAR(50),
    payment_terms VARCHAR(100),
    lead_time_days INT COMMENT 'Average lead time in days',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_vendor_active (is_active)
);

-- Vendor products (products supplied by each vendor)
CREATE TABLE vendor_products (
    vendor_product_id INT AUTO_INCREMENT PRIMARY KEY,
    vendor_id INT NOT NULL,
    product_id INT NOT NULL,
    vendor_product_code VARCHAR(50) COMMENT 'Vendor-specific product code',
    purchase_price DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    min_order_quantity DECIMAL(10,3) NOT NULL,
    is_preferred BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (vendor_id) REFERENCES vendors(vendor_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    UNIQUE KEY uk_vendor_product (vendor_id, product_id),
    INDEX idx_vendor_product (product_id, vendor_id)
);

-- Purchase orders
CREATE TABLE purchase_orders (
    po_id INT AUTO_INCREMENT PRIMARY KEY,
    po_number VARCHAR(50) NOT NULL,
    vendor_id INT NOT NULL,
    order_date DATE NOT NULL,
    expected_delivery_date DATE,
    status ENUM('draft', 'pending', 'approved', 'shipped', 'received', 'cancelled', 'partially_received') NOT NULL DEFAULT 'draft',
    total_amount DECIMAL(12,2) NOT NULL,
    notes TEXT,
    created_by INT NOT NULL COMMENT 'User ID who created the PO',
    approved_by INT COMMENT 'User ID who approved the PO',
    approved_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (vendor_id) REFERENCES vendors(vendor_id),
    UNIQUE KEY uk_po_number (po_number),
    INDEX idx_po_vendor (vendor_id),
    INDEX idx_po_status (status),
    INDEX idx_po_dates (order_date, expected_delivery_date)
);

-- Purchase order items
CREATE TABLE purchase_order_items (
    po_item_id INT AUTO_INCREMENT PRIMARY KEY,
    po_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity_ordered DECIMAL(10,3) NOT NULL,
    quantity_received DECIMAL(10,3) DEFAULT 0,
    unit_price DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    expected_delivery_date DATE,
    received_date DATE,
    status ENUM('pending', 'partially_received', 'received', 'cancelled') NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (po_id) REFERENCES purchase_orders(po_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    INDEX idx_po_item_product (product_id),
    INDEX idx_po_item_status (status)
);

-- Inventory batches (for FIFO/FEFO tracking)
CREATE TABLE inventory_batches (
    batch_id INT AUTO_INCREMENT PRIMARY KEY,
    product_id INT NOT NULL,
    po_item_id INT NOT NULL,
    location_id INT NOT NULL,
    batch_number VARCHAR(50) NOT NULL,
    quantity_received DECIMAL(10,3) NOT NULL,
    quantity_available DECIMAL(10,3) NOT NULL,
    unit_cost DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    manufacture_date DATE,
    expiration_date DATE,
    received_date DATE NOT NULL,
    received_by INT NOT NULL COMMENT 'User ID who received the batch',
    status ENUM('available', 'reserved', 'consumed', 'expired', 'damaged') NOT NULL DEFAULT 'available',
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    FOREIGN KEY (po_item_id) REFERENCES purchase_order_items(po_item_id),
    FOREIGN KEY (location_id) REFERENCES locations(location_id),
    INDEX idx_batch_product (product_id),
    INDEX idx_batch_location (location_id),
    INDEX idx_batch_expiration (expiration_date),
    INDEX idx_batch_status (status)
);

-- Branch order requests
CREATE TABLE branch_orders (
    order_id INT AUTO_INCREMENT PRIMARY KEY,
    order_number VARCHAR(50) NOT NULL,
    branch_id INT NOT NULL COMMENT 'Requesting branch location',
    warehouse_id INT NOT NULL COMMENT 'Fulfilling warehouse location',
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    required_date DATE NOT NULL,
    status ENUM('draft', 'pending', 'processing', 'picking', 'packed', 'shipped', 'delivered', 'cancelled', 'partially_delivered') NOT NULL DEFAULT 'draft',
    total_items INT NOT NULL DEFAULT 0,
    total_quantity DECIMAL(12,3) NOT NULL DEFAULT 0,
    notes TEXT,
    created_by INT NOT NULL COMMENT 'User ID who created the order',
    approved_by INT COMMENT 'User ID who approved the order',
    approved_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (branch_id) REFERENCES locations(location_id),
    FOREIGN KEY (warehouse_id) REFERENCES locations(location_id),
    UNIQUE KEY uk_order_number (order_number),
    INDEX idx_branch_order_branch (branch_id),
    INDEX idx_branch_order_warehouse (warehouse_id),
    INDEX idx_branch_order_status (status),
    INDEX idx_branch_order_dates (order_date, required_date)
);

-- Branch order items
CREATE TABLE branch_order_items (
    order_item_id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT NOT NULL,
    product_id INT NOT NULL,
    quantity_requested DECIMAL(10,3) NOT NULL,
    quantity_allocated DECIMAL(10,3) DEFAULT 0,
    quantity_picked DECIMAL(10,3) DEFAULT 0,
    quantity_shipped DECIMAL(10,3) DEFAULT 0,
    quantity_received DECIMAL(10,3) DEFAULT 0,
    status ENUM('pending', 'allocated', 'picked', 'packed', 'shipped', 'delivered', 'cancelled', 'partially_delivered') NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES branch_orders(order_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    INDEX idx_order_item_product (product_id),
    INDEX idx_order_item_status (status)
);

-- Batch allocations for branch orders (FIFO/FEFO implementation)
CREATE TABLE branch_order_allocations (
    allocation_id INT AUTO_INCREMENT PRIMARY KEY,
    order_item_id INT NOT NULL,
    batch_id INT NOT NULL,
    quantity_allocated DECIMAL(10,3) NOT NULL,
    allocation_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    allocated_by INT NOT NULL COMMENT 'User ID who made the allocation',
    status ENUM('allocated', 'picked', 'shipped', 'delivered', 'cancelled') NOT NULL DEFAULT 'allocated',
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (order_item_id) REFERENCES branch_order_items(order_item_id),
    FOREIGN KEY (batch_id) REFERENCES inventory_batches(batch_id),
    INDEX idx_allocation_order_item (order_item_id),
    INDEX idx_allocation_batch (batch_id),
    INDEX idx_allocation_status (status)
);

-- Picking lists
CREATE TABLE picking_lists (
    picking_list_id INT AUTO_INCREMENT PRIMARY KEY,
    order_id INT NOT NULL,
    picking_list_number VARCHAR(50) NOT NULL,
    picker_id INT NOT NULL COMMENT 'User ID who performed the picking',
    picking_start_time TIMESTAMP NULL,
    picking_end_time TIMESTAMP NULL,
    status ENUM('pending', 'in_progress', 'completed', 'cancelled') NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES branch_orders(order_id),
    UNIQUE KEY uk_picking_list_number (picking_list_number),
    INDEX idx_picking_list_order (order_id),
    INDEX idx_picking_list_status (status)
);

-- Picking list items
CREATE TABLE picking_list_items (
    picking_item_id INT AUTO_INCREMENT PRIMARY KEY,
    picking_list_id INT NOT NULL,
    order_item_id INT NOT NULL,
    batch_id INT NOT NULL,
    quantity_to_pick DECIMAL(10,3) NOT NULL,
    quantity_picked DECIMAL(10,3) DEFAULT 0,
    location_code VARCHAR(50) NOT NULL COMMENT 'Physical location in warehouse',
    status ENUM('pending', 'picked', 'partially_picked', 'cancelled') NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (picking_list_id) REFERENCES picking_lists(picking_list_id),
    FOREIGN KEY (order_item_id) REFERENCES branch_order_items(order_item_id),
    FOREIGN KEY (batch_id) REFERENCES inventory_batches(batch_id),
    INDEX idx_picking_item_order (order_item_id),
    INDEX idx_picking_item_batch (batch_id),
    INDEX idx_picking_item_status (status)
);

-- Delivery tracking
CREATE TABLE deliveries (
    delivery_id INT AUTO_INCREMENT PRIMARY KEY,
    delivery_number VARCHAR(50) NOT NULL,
    order_id INT NOT NULL,
    driver_name VARCHAR(100),
    vehicle_number VARCHAR(50),
    dispatch_time TIMESTAMP NULL,
    estimated_arrival TIMESTAMP NULL,
    actual_arrival TIMESTAMP NULL,
    status ENUM('pending', 'dispatched', 'in_transit', 'delivered', 'cancelled') NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (order_id) REFERENCES branch_orders(order_id),
    UNIQUE KEY uk_delivery_number (delivery_number),
    INDEX idx_delivery_order (order_id),
    INDEX idx_delivery_status (status)
);

-- Stock movements (transfers between locations)
CREATE TABLE stock_movements (
    movement_id INT AUTO_INCREMENT PRIMARY KEY,
    movement_type ENUM('transfer', 'adjustment', 'consumption', 'return', 'loss') NOT NULL,
    product_id INT NOT NULL,
    batch_id INT,
    from_location_id INT,
    to_location_id INT,
    quantity DECIMAL(10,3) NOT NULL,
    movement_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    reference_number VARCHAR(50) COMMENT 'PO, Order, or other reference number',
    reason_code VARCHAR(50) COMMENT 'Reason for adjustment/loss etc.',
    notes TEXT,
    created_by INT NOT NULL COMMENT 'User ID who recorded the movement',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    FOREIGN KEY (batch_id) REFERENCES inventory_batches(batch_id),
    FOREIGN KEY (from_location_id) REFERENCES locations(location_id),
    FOREIGN KEY (to_location_id) REFERENCES locations(location_id),
    INDEX idx_movement_product (product_id),
    INDEX idx_movement_batch (batch_id),
    INDEX idx_movement_dates (movement_date),
    INDEX idx_movement_type (movement_type)
);

-- Stock consumption (usage at branch locations)
CREATE TABLE stock_consumption (
    consumption_id INT AUTO_INCREMENT PRIMARY KEY,
    branch_id INT NOT NULL,
    product_id INT NOT NULL,
    batch_id INT,
    consumption_date DATE NOT NULL,
    quantity DECIMAL(10,3) NOT NULL,
    unit_cost DECIMAL(10,2) NOT NULL COMMENT 'Cost at time of consumption',
    reason ENUM('regular_use', 'waste', 'spoilage', 'theft', 'other') NOT NULL DEFAULT 'regular_use',
    reference_number VARCHAR(50) COMMENT 'Recipe, waste log, etc.',
    notes TEXT,
    recorded_by INT NOT NULL COMMENT 'User ID who recorded the consumption',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (branch_id) REFERENCES locations(location_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    FOREIGN KEY (batch_id) REFERENCES inventory_batches(batch_id),
    INDEX idx_consumption_branch (branch_id),
    INDEX idx_consumption_product (product_id),
    INDEX idx_consumption_date (consumption_date)
);

-- Inventory counts (physical inventory)
CREATE TABLE inventory_counts (
    count_id INT AUTO_INCREMENT PRIMARY KEY,
    location_id INT NOT NULL,
    count_date DATE NOT NULL,
    count_type ENUM('full', 'partial', 'cycle') NOT NULL,
    status ENUM('planned', 'in_progress', 'completed', 'adjusted', 'cancelled') NOT NULL DEFAULT 'planned',
    started_by INT NOT NULL COMMENT 'User ID who started the count',
    completed_by INT COMMENT 'User ID who completed the count',
    completed_at TIMESTAMP NULL,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (location_id) REFERENCES locations(location_id),
    INDEX idx_count_location (location_id),
    INDEX idx_count_status (status),
    INDEX idx_count_date (count_date)
);

-- Inventory count items
CREATE TABLE inventory_count_items (
    count_item_id INT AUTO_INCREMENT PRIMARY KEY,
    count_id INT NOT NULL,
    product_id INT NOT NULL,
    batch_id INT,
    system_quantity DECIMAL(10,3) NOT NULL COMMENT 'System quantity at time of count',
    counted_quantity DECIMAL(10,3) NOT NULL,
    variance DECIMAL(10,3) GENERATED ALWAYS AS (counted_quantity - system_quantity) STORED,
    unit_cost DECIMAL(10,2) NOT NULL COMMENT 'Cost at time of count',
    notes TEXT,
    counted_by INT NOT NULL COMMENT 'User ID who performed the count',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (count_id) REFERENCES inventory_counts(count_id),
    FOREIGN KEY (product_id) REFERENCES products(product_id),
    FOREIGN KEY (batch_id) REFERENCES inventory_batches(batch_id),
    INDEX idx_count_item_product (product_id),
    INDEX idx_count_item_batch (batch_id)
);

-- Audit logs
CREATE TABLE audit_logs (
    log_id INT AUTO_INCREMENT PRIMARY KEY,
    table_name VARCHAR(50) NOT NULL,
    record_id INT NOT NULL,
    action ENUM('insert', 'update', 'delete') NOT NULL,
    old_values JSON COMMENT 'JSON representation of old values',
    new_values JSON COMMENT 'JSON representation of new values',
    changed_by INT NOT NULL COMMENT 'User ID who made the change',
    changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_audit_table_record (table_name, record_id),
    INDEX idx_audit_action (action),
    INDEX idx_audit_timestamp (changed_at)
);

-- Current inventory view (simplified for reporting)
CREATE VIEW current_inventory AS
SELECT 
    b.product_id,
    p.product_code,
    p.product_name,
    b.location_id,
    l.location_name,
    l.location_type,
    b.batch_id,
    b.batch_number,
    b.expiration_date,
    SUM(b.quantity_available) AS quantity_available,
    AVG(b.unit_cost) AS avg_unit_cost,
    MIN(b.expiration_date) AS earliest_expiration
FROM 
    inventory_batches b
JOIN 
    products p ON b.product_id = p.product_id
JOIN 
    locations l ON b.location_id = l.location_id
WHERE 
    b.status = 'available'
GROUP BY 
    b.product_id, b.location_id, b.batch_id, b.batch_number, b.expiration_date,
    p.product_code, p.product_name, l.location_name, l.location_type;