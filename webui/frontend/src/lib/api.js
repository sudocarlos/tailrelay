/**
 * Shared API client for all backend requests.
 * Handles JSON parsing, credentials, and error messages.
 */
export async function fetchJSON(url, options = {}) {
  const response = await fetch(url, {
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers || {}),
    },
    ...options,
  });

  if (!response.ok) {
    let message = '';
    try {
      const data = await response.json();
      message = data.error || data.message || `Request failed`;
    } catch {
      message = await response.text();
    }
    throw new Error(`[${response.status}] ${message || 'Request failed'}`);
  }

  if (response.status === 204 || response.headers.get('Content-Length') === '0') {
    return null;
  }

  return response.json();
}

/**
 * POST with FormData (for file uploads). Does NOT set Content-Type
 * so the browser can set the multipart boundary automatically.
 */
export async function postFormData(url, formData) {
  const response = await fetch(url, {
    method: 'POST',
    credentials: 'same-origin',
    body: formData,
  });

  if (!response.ok) {
    let message = '';
    try {
      const data = await response.json();
      message = data.error || data.message || `Request failed`;
    } catch {
      message = await response.text();
    }
    throw new Error(`[${response.status}] ${message || 'Request failed'}`);
  }

  if (response.status === 204 || response.headers.get('Content-Length') === '0') {
    return null;
  }

  return response.json();
}
