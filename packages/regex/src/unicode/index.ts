import { union, type Ranges } from "../program/ranges";
import { PROPERTIES, SPACE } from "./tables";

export const hasProperty = (name: string): boolean => Object.hasOwn(PROPERTIES, name);

export const propertyRanges = (name: string): Ranges => PROPERTIES[name];

export const PERL: Record<"d" | "w" | "s", Ranges> = {
  d: PROPERTIES.Nd,
  w: union(union(PROPERTIES.L, PROPERTIES.N), [95, 95]),
  s: SPACE,
};
