import { createSign } from "node:crypto";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { ApiError } from "./api-error";
import { mp3Problem } from "./audio";
import type { Config } from "./config";

const ENDPOINT = "https://texttospeech.googleapis.com/v1/text:synthesize";
const SCOPE = "https://www.googleapis.com/auth/cloud-platform";
const TOKEN_URL = "https://oauth2.googleapis.com/token";

interface Credentials {
  type: "service_account" | "authorized_user";
  client_email?: string;
  private_key?: string;
  token_uri?: string;
  client_id?: string;
  client_secret?: string;
  refresh_token?: string;
}

function readCredentials(source: string): Credentials {
  let raw: string;
  if (source.trim().startsWith("{")) raw = source;
  else {
    const file = source
      ? source.replace(/^~(?=\/)/, os.homedir())
      : path.join(process.env.XDG_CONFIG_HOME || path.join(os.homedir(), ".config"), "gcloud", "application_default_credentials.json");
    try {
      raw = fs.readFileSync(file, "utf8");
    } catch {
      throw new ApiError("There are no Google credentials. Set google_credentials_file in config.toml or GOOGLE_APPLICATION_CREDENTIALS.");
    }
  }
  try {
    return JSON.parse(raw) as Credentials;
  } catch {
    throw new ApiError("The Google credentials are not valid JSON.");
  }
}

const b64url = (v: string | Buffer) => Buffer.from(v).toString("base64url");

async function accessToken(source: string, fetchImpl: typeof fetch): Promise<string> {
  const c = readCredentials(source);
  let body: URLSearchParams;
  let url = TOKEN_URL;
  if (c.type === "service_account" && c.client_email && c.private_key) {
    url = c.token_uri || TOKEN_URL;
    const iat = Math.floor(Date.now() / 1000);
    const unsigned = `${b64url(JSON.stringify({ alg: "RS256", typ: "JWT" }))}.${b64url(
      JSON.stringify({ iss: c.client_email, scope: SCOPE, aud: url, iat, exp: iat + 3600 }),
    )}`;
    let signature: Buffer;
    try {
      signature = createSign("RSA-SHA256").update(unsigned).sign(c.private_key);
    } catch {
      throw new ApiError("The Google service account key could not be used to sign a request.");
    }
    body = new URLSearchParams({ grant_type: "urn:ietf:params:oauth:grant-type:jwt-bearer", assertion: `${unsigned}.${b64url(signature)}` });
  } else if (c.type === "authorized_user" && c.refresh_token) {
    body = new URLSearchParams({
      grant_type: "refresh_token", refresh_token: c.refresh_token, client_id: c.client_id ?? "", client_secret: c.client_secret ?? "",
    });
  } else {
    throw new ApiError("The Google credentials are neither a service account key nor application default credentials.");
  }
  let res: Response;
  try {
    res = await fetchImpl(url, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body });
  } catch {
    throw new ApiError("Could not reach Google. Check the network connection and try again.");
  }
  if (!res.ok) throw new ApiError(`Google rejected the credentials (${res.status}). Check google_credentials_file in config.toml.`);
  const token = ((await res.json()) as { access_token?: string }).access_token;
  if (!token) throw new ApiError("Google did not return an access token.");
  return token;
}

function httpProblem(status: number): string {
  if (status === 400) return "Google refused the text or voice (400). Check the Arabic text and tts_voice.";
  if (status === 401) return "Google rejected the credentials (401). Check google_credentials_file in config.toml.";
  if (status === 403) return "Google denied the request or the quota is used up (403).";
  if (status === 429) return "Google is rate limiting requests (429). Try again in a moment.";
  if (status >= 500) return `Google had a server error (${status}). Try again later.`;
  return `Google refused the request (${status}).`;
}

/** One synthesis call for one piece of Arabic text. Throws ApiError; never retries. */
export async function synthesize(
  text: string,
  cfg: Pick<Config, "googleCredentials" | "ttsVoice" | "audioRate">,
  fetchImpl: typeof fetch = fetch,
): Promise<Buffer> {
  const token = await accessToken(cfg.googleCredentials, fetchImpl);
  let res: Response;
  try {
    res = await fetchImpl(ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({
        input: { text },
        voice: { languageCode: "ar-XA", name: cfg.ttsVoice },
        audioConfig: { audioEncoding: "MP3", sampleRateHertz: 48000, speakingRate: cfg.audioRate },
      }),
    });
  } catch {
    throw new ApiError("Could not reach Google. Check the network connection and try again.");
  }
  if (!res.ok) throw new ApiError(httpProblem(res.status));
  const content = ((await res.json().catch(() => ({}))) as { audioContent?: string }).audioContent;
  const bytes = Buffer.from(content ?? "", "base64");
  const problem = mp3Problem(bytes);
  if (problem) throw new ApiError(`Google's audio was not saved. ${problem}`);
  return bytes;
}
