// Who runs this copy of SentryAtlas, read at build time, so a fork never publishes
// the original operator's name or address as its own.
export const OPERATOR = process.env.SITE_OPERATOR?.trim() || undefined;
export const PRIVACY_EMAIL = process.env.SITE_PRIVACY_EMAIL?.trim() || undefined;
