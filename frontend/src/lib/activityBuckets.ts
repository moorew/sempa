// Activity buckets — a lightweight, offline classifier that recognises the
// *kind* of work a task is from its title (and tags) and assigns a sensible
// default duration. This is the instant, zero-latency layer behind the
// "auto-categorize" time prediction: the local AI can refine the number, but we
// never block the UI waiting for it. Buckets also give the completion prompt a
// human label ("looks like ✉️ Email") and a starting time to one-tap confirm.

export interface ActivityBucket {
  key: string;
  label: string;
  emoji: string;
  defaultMinutes: number;
  keywords: string[];
}

// Ordered by specificity: earlier, more-distinctive buckets win ties. "Other" is
// the catch-all and matches nothing by keyword.
export const ACTIVITY_BUCKETS: ActivityBucket[] = [
  { key: 'meeting', label: 'Meeting', emoji: '📅', defaultMinutes: 30,
    keywords: ['meeting', 'meet ', 'sync', 'standup', 'stand-up', '1:1', 'one-on-one', 'zoom', 'call with', 'interview', 'catchup', 'catch up', 'check-in'] },
  { key: 'email', label: 'Email / messages', emoji: '✉️', defaultMinutes: 15,
    keywords: ['email', 'e-mail', 'reply', 'respond', 'inbox', 'message', 'dm ', 'slack', 'follow up', 'follow-up', 'send '] },
  { key: 'call', label: 'Call', emoji: '📞', defaultMinutes: 15,
    keywords: ['call ', 'phone', 'ring ', 'dial'] },
  { key: 'review', label: 'Review / read', emoji: '👀', defaultMinutes: 30,
    keywords: ['review', 'read ', 'reading', 'check ', 'proofread', 'feedback', 'look over', 'go through', 'pr ', 'approve'] },
  { key: 'writing', label: 'Writing', emoji: '✍️', defaultMinutes: 45,
    keywords: ['write', 'draft', 'document', 'doc ', 'notes', 'blog', 'post ', 'report', 'proposal', 'spec', 'summary'] },
  { key: 'deepwork', label: 'Deep work', emoji: '🧠', defaultMinutes: 90,
    keywords: ['build', 'code', 'develop', 'design', 'implement', 'research', 'analyze', 'analyse', 'create', 'figure out', 'fix', 'debug', 'refactor', 'architect'] },
  { key: 'admin', label: 'Admin', emoji: '🗂️', defaultMinutes: 20,
    keywords: ['admin', 'expense', 'invoice', 'form', 'paperwork', 'submit', 'file ', 'pay ', 'renew', 'book ', 'schedule', 'sign up', 'register', 'update ', 'organize', 'organise', 'sort'] },
  { key: 'errand', label: 'Errand / chore', emoji: '🧺', defaultMinutes: 30,
    keywords: ['buy', 'pick up', 'pickup', 'grocery', 'groceries', 'shop', 'drop off', 'clean', 'tidy', 'laundry', 'dishes', 'cook', 'meal', 'fix the', 'water', 'walk the'] },
  { key: 'personal', label: 'Personal / health', emoji: '🌿', defaultMinutes: 30,
    keywords: ['gym', 'workout', 'exercise', 'run ', 'walk', 'meditate', 'meditation', 'doctor', 'dentist', 'appointment', 'self-care', 'journal', 'stretch'] },
  { key: 'planning', label: 'Planning', emoji: '🗓️', defaultMinutes: 20,
    keywords: ['plan ', 'planning', 'prepare', 'prep ', 'outline', 'brainstorm', 'organize the', 'roadmap', 'agenda', 'prioritize'] },
  { key: 'other', label: 'Other', emoji: '○', defaultMinutes: 30, keywords: [] },
];

export const OTHER_BUCKET = ACTIVITY_BUCKETS[ACTIVITY_BUCKETS.length - 1];

export function bucketByKey(key: string): ActivityBucket {
  return ACTIVITY_BUCKETS.find((b) => b.key === key) ?? OTHER_BUCKET;
}

/**
 * Classify a task into an activity bucket from its title and tags. Keyword
 * matching only — instant and offline. Tags that name a bucket (e.g. an "email"
 * tag) are honoured first since they're an explicit signal.
 */
export function classifyActivity(title: string, tags: string[] = []): ActivityBucket {
  const t = ` ${title.toLowerCase()} `;
  // Explicit tag → bucket match wins.
  const tagSet = tags.map((x) => x.toLowerCase());
  for (const b of ACTIVITY_BUCKETS) {
    if (b.key === 'other') continue;
    if (tagSet.includes(b.key) || tagSet.includes(b.label.toLowerCase())) return b;
  }
  for (const b of ACTIVITY_BUCKETS) {
    if (b.key === 'other') continue;
    if (b.keywords.some((kw) => t.includes(kw))) return b;
  }
  return OTHER_BUCKET;
}
