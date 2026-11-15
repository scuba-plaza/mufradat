import { PROPERTIES } from "./tables";

export const hasProperty = (name: string): boolean => Object.hasOwn(PROPERTIES, name);
