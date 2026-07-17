ALTER TABLE contribution_withdrawals
    ADD COLUMN IF NOT EXISTS payment_qr_code TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN contribution_withdrawals.payment_qr_code IS
    'Base64 data URL for the user-provided Alipay or WeChat payment QR code; only exposed through authenticated endpoints';
