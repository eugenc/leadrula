-- Unified routing: extend route origin/destination enums.
-- Enum values must be committed before use (see 0064_unified_routes_apply).

ALTER TYPE route_origin ADD VALUE IF NOT EXISTS 'webhook';
ALTER TYPE route_origin ADD VALUE IF NOT EXISTS 'integration';

ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'contract';
ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'pipeline';
ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'webhook';
ALTER TYPE route_destination ADD VALUE IF NOT EXISTS 'integration';
