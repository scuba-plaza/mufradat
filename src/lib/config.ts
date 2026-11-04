import fs from "node:fs";
import os from "node:os";
import path from "node:path";

export interface Config {
  workspace: string;
  mistralApiKey: string;
  mistralModel: string;
  /** Sent to Mistral as `reasoning_effort` when not empty; unset, the model answers without reasoning. */
  mistralReasoningEffort: string;
  googleCredentials: string;
  ttsVoice: string;
  audioRate: number;
  deckName: string;
  deckId: number;
  modelId: number;
}

const home = () => os.homedir();
const configPath = () => path.join(process.env.XDG_CONFIG_HOME || path.join(home(), ".config"), "mufradat", "config.toml");

/** Reads `key = "text"`, `key = 12` and comment lines. */
function parseToml(text: string): Record<string, string | number> {
  const out: Record<string, string | number> = {};
  text.split("\n").forEach((raw, i) => {
    const line = raw.trim();
    if (line === "" || line.startsWith("#")) return;
    const m = line.match(/^([A-Za-z_][\w]*)\s*=\s*(?:"((?:[^"\\]|\\.)*)"|(-?\d+(?:\.\d+)?))\s*(?:#.*)?$/);
    if (!m) throw new Error(`config.toml line ${i + 1} is not understood: ${line.split("=")[0].trim()}`);
    out[m[1]] = m[2] !== undefined ? m[2].replace(/\\(["\\])/g, "$1") : Number(m[3]);
  });
  return out;
}

const expand = (p: string) => (p.startsWith("~/") ? path.join(home(), p.slice(2)) : p);

export function loadConfig(env: NodeJS.ProcessEnv = process.env): Config {
  const file = configPath();
  let values: Record<string, string | number> = {};
  if (fs.existsSync(file)) {
    if ((fs.statSync(file).mode & 0o077) !== 0) {
      throw new Error(`${file} must be readable only by you. Run: chmod 600 ${file}`);
    }
    values = parseToml(fs.readFileSync(file, "utf8"));
  }
  const text = (key: string, fallback: string) => (typeof values[key] === "string" ? (values[key] as string) : fallback);
  const num = (key: string, fallback: number) => (typeof values[key] === "number" ? (values[key] as number) : fallback);
  return {
    workspace: expand(env.MUFRADAT_WORKSPACE || text("workspace", path.join(home(), ".local", "share", "mufradat"))),
    mistralApiKey: env.MISTRAL_API_KEY || text("mistral_api_key", ""),
    mistralModel: text("mistral_model", "mistral-medium-latest"),
    mistralReasoningEffort: text("mistral_reasoning_effort", ""),
    googleCredentials: env.GOOGLE_APPLICATION_CREDENTIALS || text("google_credentials_file", ""),
    ttsVoice: text("tts_voice", "ar-XA-Chirp3-HD-Achernar"),
    audioRate: num("audio_rate", 0.9),
    deckName: text("deck_name", "Arabic MSA Core"),
    deckId: num("deck_id", 1_700_000_000_001),
    modelId: num("model_id", 1_700_000_000_002),
  };
}
