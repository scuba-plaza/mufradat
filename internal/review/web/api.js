export class ApiError extends Error {
  /**
   * @param {string} message
   * @param {number} status
   * @param {import('./types.js').Result | null} body
   */
  constructor(message, status, body) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

/**
 * @param {string} path
 * @param {RequestInit} [options]
 * @returns {Promise<unknown>}
 */
async function request(path, options) {
  const res = await fetch(path, options);
  /** @type {import('./types.js').Result | null} */
  let body = null;
  try {
    body = await res.json();
  } catch {
    body = null;
  }
  if (!res.ok) {
    throw new ApiError(body && body.message ? body.message : res.status + ' ' + res.statusText, res.status, body);
  }
  return body;
}

/**
 * @param {string} id
 * @returns {Promise<import('./types.js').View>}
 */
export async function loadState(id) {
  return /** @type {import('./types.js').View} */ (await request('api/state?id=' + encodeURIComponent(id)));
}

/**
 * @returns {Promise<import('./types.js').VoiceList>}
 */
export async function loadVoices() {
  return /** @type {import('./types.js').VoiceList} */ (await request('api/voices'));
}

/**
 * @param {string} action
 * @param {string} id
 * @param {Record<string, unknown>} [body]
 * @returns {Promise<import('./types.js').Result>}
 */
export async function post(action, id, body) {
  return /** @type {import('./types.js').Result} */ (
    await request('api/' + action + '?id=' + encodeURIComponent(id), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body || {}),
    })
  );
}

/**
 * @returns {Promise<import('./types.js').Summary>}
 */
export async function finish() {
  return /** @type {import('./types.js').Summary} */ (
    await request('api/finish', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' })
  );
}

/**
 * @param {string} id
 * @param {string} kind
 * @returns {string}
 */
export function audioURL(id, kind) {
  return 'audio?id=' + encodeURIComponent(id) + '&clip=' + kind + '&v=' + Date.now();
}

/**
 * @returns {Promise<import('./types.js').Comparison>}
 */
export async function loadComparison() {
  return /** @type {import('./types.js').Comparison} */ (await request('api/compare'));
}

/**
 * @param {string} voice
 * @returns {Promise<import('./types.js').Result>}
 */
export async function use(voice) {
  return /** @type {import('./types.js').Result} */ (
    await request('api/use', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ voice }),
    })
  );
}

/**
 * @param {string} voice
 * @param {string} id
 * @returns {string}
 */
export function sampleURL(voice, id) {
  return 'sample?voice=' + encodeURIComponent(voice) + '&id=' + encodeURIComponent(id);
}
