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
    const message = await response.text();
    throw new Error(message || `Request failed: ${response.status}`);
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
    const message = await response.text();
    throw new Error(message || `Request failed: ${response.status}`);
  }

  return response.json();
}
