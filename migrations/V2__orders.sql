CREATE TYPE order_status AS ENUM (
  'CREATED', 'PAYMENT_PENDING', 'PAID', 'SHIPPED', 'COMPLETED', 'CANCELED'
);

CREATE TYPE promo_discount_type AS ENUM ('PERCENTAGE', 'FIXED_AMOUNT');

CREATE TYPE user_operation_type AS ENUM ('CREATE_ORDER', 'UPDATE_ORDER');

CREATE TABLE promo_codes (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  code VARCHAR(20) NOT NULL UNIQUE,
  discount_type promo_discount_type NOT NULL,
  discount_value NUMERIC(12,2) NOT NULL CHECK (discount_value > 0),
  min_order_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
  max_uses INTEGER NOT NULL CHECK (max_uses > 0),
  current_uses INTEGER NOT NULL DEFAULT 0,
  valid_from TIMESTAMP NOT NULL,
  valid_until TIMESTAMP NOT NULL,
  active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE orders (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL,
  status order_status NOT NULL DEFAULT 'CREATED',
  promo_code_id UUID REFERENCES promo_codes(id),
  total_amount NUMERIC(12,2) NOT NULL,
  discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_user_status ON orders(user_id, status);

CREATE TABLE order_items (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  order_id UUID NOT NULL REFERENCES orders(id),
  product_id UUID NOT NULL REFERENCES products(id),
  quantity INTEGER NOT NULL CHECK (quantity >= 1),
  price_at_order NUMERIC(12,2) NOT NULL
);

CREATE INDEX idx_order_items_order ON order_items(order_id);

CREATE TABLE user_operations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  user_id UUID NOT NULL,
  operation_type user_operation_type NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_operations_user ON user_operations(user_id, operation_type, created_at DESC);

CREATE TRIGGER trg_orders_updated_at
BEFORE UPDATE ON orders
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_promo_codes_updated_at
BEFORE UPDATE ON promo_codes
FOR EACH ROW EXECUTE FUNCTION set_updated_at();