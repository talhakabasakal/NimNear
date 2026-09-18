// Next.js inlines process.env.NEXT_PUBLIC_* and process.env.NODE_ENV only on
// static member access. Keep these reads at module scope and do not import this
// file from next.config.ts — config evaluation is Node, not the client bundle.
export const PUBLIC_NIMIQ_NETWORK = process.env.NEXT_PUBLIC_NIMNEAR_NIMIQ_NETWORK;
export const PUBLIC_NIMIQ_HUB_ENABLED = process.env.NEXT_PUBLIC_NIMNEAR_HUB_ENABLED;
export const PUBLIC_NODE_ENV = process.env.NODE_ENV;
