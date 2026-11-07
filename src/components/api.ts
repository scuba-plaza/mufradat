export async function refusal(res: Response): Promise<Error> {
  const body = await res.json().catch(() => ({}));
  const message = body.error ?? (Array.isArray(body.errors) ? body.errors.join(" ") : `Request failed (${res.status}).`);
  return Object.assign(new Error(message), { errors: body.errors as string[] | undefined });
}

/** Calls the app's own API. Throws with the server's message when it refuses. */
export async function api<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  if (!res.ok) throw await refusal(res);
  return (await res.json().catch(() => ({}))) as T;
}

export const post = <T,>(url: string, body: unknown) =>
  api<T>(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
