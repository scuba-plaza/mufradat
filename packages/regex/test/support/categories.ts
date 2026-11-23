import { CATEGORIES } from "../../scripts/generate-unicode";

export { CATEGORIES };

export const nativeProperty = (name: string): string => (CATEGORIES.includes(name) ? name : `Script=${name}`);
