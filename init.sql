-- 会员积分系统：结构与演示数据（500 会员 + 约 12 个月流水）

BEGIN;

DROP TABLE IF EXISTS point_transactions CASCADE;
DROP TABLE IF EXISTS members CASCADE;
DROP TABLE IF EXISTS point_rules CASCADE;
DROP TABLE IF EXISTS tier_configs CASCADE;

CREATE TABLE tier_configs (
  id SERIAL PRIMARY KEY,
  tier_code VARCHAR(16) NOT NULL UNIQUE,
  tier_name VARCHAR(32) NOT NULL,
  min_lifetime_points BIGINT NOT NULL DEFAULT 0
);

COMMENT ON TABLE tier_configs IS '各等级所需的累计积分门槛（由高到低匹配第一条满足）';

CREATE TABLE point_rules (
  id SERIAL PRIMARY KEY,
  spend_yuan_per_point NUMERIC(14, 2) NOT NULL DEFAULT 10,
  birthday_multiplier INT NOT NULL DEFAULT 2,
  campaign_multiplier INT NOT NULL DEFAULT 1,
  campaign_end DATE,
  expiry_jan_clear_after_years INT NOT NULL DEFAULT 2,
  active_event_name VARCHAR(200) NOT NULL DEFAULT ''
);

COMMENT ON COLUMN point_rules.spend_yuan_per_point IS '每消费若干元可获得 1 分（向下取整）';
COMMENT ON COLUMN point_rules.expiry_jan_clear_after_years IS '积分按自然入账年：expires_at = 入账年次年 + 若干年后的 1 月 1 日';

CREATE TABLE members (
  id BIGSERIAL PRIMARY KEY,
  phone VARCHAR(20) NOT NULL UNIQUE,
  name VARCHAR(100) NOT NULL,
  birthday DATE,
  registered_at DATE NOT NULL DEFAULT CURRENT_DATE,
  tier_code VARCHAR(16) NOT NULL REFERENCES tier_configs (tier_code),
  lifetime_points BIGINT NOT NULL DEFAULT 0,
  points_balance BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_members_phone ON members (phone);
CREATE INDEX idx_members_tier ON members (tier_code);

CREATE TABLE point_transactions (
  id BIGSERIAL PRIMARY KEY,
  member_id BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
  direction VARCHAR(3) NOT NULL CHECK (direction IN ('in', 'out')),
  points BIGINT NOT NULL CHECK (points > 0),
  points_delta BIGINT NOT NULL CHECK (points_delta <> 0),
  source_type VARCHAR(32) NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  operator VARCHAR(64) NOT NULL DEFAULT '',
  money_amount NUMERIC(14, 2),
  product_name VARCHAR(200) NOT NULL DEFAULT '',
  redeem_amount NUMERIC(14, 2),
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at DATE
);

CREATE INDEX idx_pt_member_time ON point_transactions (member_id, occurred_at DESC);
CREATE INDEX idx_pt_occurred ON point_transactions (occurred_at);

INSERT INTO tier_configs (tier_code, tier_name, min_lifetime_points) VALUES
  ('normal', '普通', 0),
  ('silver', '银卡', 5000),
  ('gold', '金卡', 20000),
  ('platinum', '铂金', 50000);

INSERT INTO point_rules (id, spend_yuan_per_point, birthday_multiplier, campaign_multiplier, campaign_end, expiry_jan_clear_after_years, active_event_name)
VALUES (1, 10, 2, 2, CURRENT_DATE + 60, 2, '春季加倍活动');

-- 500 名会员
INSERT INTO members (phone, name, birthday, registered_at, tier_code, lifetime_points, points_balance)
SELECT
  '13' || LPAD((880000000 + gs)::TEXT, 9, '0'),
  '会员' || gs,
  DATE '1965-01-01' + ((gs * 37) % 16000),
  CURRENT_DATE - ((gs * 11) % 1400),
  'normal',
  0,
  0
FROM generate_series(1, 500) AS gs;

-- 约 1 年流水：混合入账/消耗/系统过期
INSERT INTO point_transactions (
  member_id, direction, points, points_delta, source_type, reason, operator,
  money_amount, product_name, redeem_amount, occurred_at, expires_at
)
SELECT
  ((gs * 7919) % 500) + 1 AS member_id,
  CASE WHEN r < 0.68 THEN 'in' ELSE 'out' END AS direction,
  pts,
  CASE WHEN r < 0.68 THEN pts::BIGINT ELSE -pts::BIGINT END AS points_delta,
  CASE
    WHEN r < 0.68 AND r2 < 0.55 THEN 'consumption'
    WHEN r < 0.68 AND r2 < 0.75 THEN 'promo'
    WHEN r < 0.68 AND r2 < 0.88 THEN 'checkin'
    WHEN r < 0.68 THEN 'manual'
    WHEN r2 < 0.55 THEN 'redemption'
    WHEN r2 < 0.78 THEN 'deduct'
    ELSE 'expiry'
  END AS source_type,
  CASE
    WHEN r < 0.68 AND r2 < 0.55 THEN '门店消费获积分'
    WHEN r < 0.68 AND r2 < 0.75 THEN '活动赠送'
    WHEN r < 0.68 AND r2 < 0.88 THEN '签到奖励'
    WHEN r < 0.68 THEN '后台补发'
    WHEN r2 < 0.55 THEN '积分兑换礼品'
    WHEN r2 < 0.78 THEN '订单抵扣'
    ELSE '年度积分到期清理'
  END AS reason,
  CASE WHEN (gs % 5) = 0 THEN 'system' ELSE 'op' || ((gs % 12) + 1)::TEXT END AS operator,
  CASE WHEN r < 0.68 AND r2 < 0.55 THEN (pts * 10 + (gs % 50))::NUMERIC(14,2) ELSE NULL END AS money_amount,
  CASE WHEN r >= 0.68 AND r2 < 0.55 THEN '礼品' || ((gs % 40) + 1)::TEXT ELSE '' END AS product_name,
  CASE WHEN r >= 0.68 AND r2 < 0.78 THEN (pts / 20.0 + 1)::NUMERIC(14,2) ELSE NULL END AS redeem_amount,
  NOW() - ((gs * 13) % 365 || ' days')::INTERVAL - ((gs * 7) % 86400 || ' seconds')::INTERVAL AS occurred_at,
  NULL::DATE AS expires_at
FROM generate_series(1, 15600) AS gs
CROSS JOIN LATERAL (
  SELECT
    (random())::DOUBLE PRECISION AS r,
    (random())::DOUBLE PRECISION AS r2,
    (CASE
      WHEN gs % 9 = 0 THEN 420
      ELSE 8 + ((gs * 131) % 180)
    END)::BIGINT AS pts
) x;

-- 入账积分到期日：入账自然年年底推进 (expiry_jan_clear_after_years + 1) 年后的 1 月 1 日
UPDATE point_transactions pt
SET expires_at = (
  DATE_TRUNC('year', pt.occurred_at AT TIME ZONE 'Asia/Shanghai')
  + ((((SELECT expiry_jan_clear_after_years FROM point_rules WHERE id = 1)::INT + 1)::TEXT || ' years')::INTERVAL)
)::DATE
WHERE pt.direction = 'in';

INSERT INTO point_transactions (
  member_id, direction, points, points_delta, source_type, reason, operator, occurred_at
)
SELECT
  s.member_id,
  'in',
  ((-s.bal)::BIGINT),
  ((-s.bal)::BIGINT),
  'manual',
  '演示数据找平：随机流水偶发超额扣减',
  'system',
  NOW()
FROM (
  SELECT member_id, SUM(points_delta)::BIGINT AS bal
  FROM point_transactions
  GROUP BY member_id
  HAVING SUM(points_delta) < 0
) s;

UPDATE point_transactions pt
SET expires_at = (
  DATE_TRUNC('year', pt.occurred_at AT TIME ZONE 'Asia/Shanghai')
  + ((((SELECT expiry_jan_clear_after_years FROM point_rules WHERE id = 1)::INT + 1)::TEXT || ' years')::INTERVAL)
)::DATE
WHERE pt.direction = 'in'
  AND pt.expires_at IS NULL;

UPDATE members m
SET points_balance = COALESCE(r.bal, 0),
    lifetime_points = COALESCE(r.lp, 0)
FROM (
  SELECT
    member_id,
    SUM(points_delta)::BIGINT AS bal,
    SUM(CASE WHEN direction = 'in' THEN points ELSE 0 END)::BIGINT AS lp
  FROM point_transactions
  GROUP BY member_id
) r
WHERE m.id = r.member_id;

UPDATE members mu
SET tier_code = sx.tier_code
FROM (
  SELECT
    mm.id AS mid,
    (
      SELECT tc.tier_code
      FROM tier_configs tc
      WHERE mm.lifetime_points >= tc.min_lifetime_points
      ORDER BY tc.min_lifetime_points DESC
      LIMIT 1
    ) AS tier_code
  FROM members mm
) sx
WHERE mu.id = sx.mid;

COMMIT;
