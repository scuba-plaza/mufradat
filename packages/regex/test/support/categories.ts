export const CATEGORIES: readonly string[] = [
  "C", "Cc", "Cf", "Co", "Cs", "L", "Ll", "Lm", "Lo", "Lt", "Lu", "M", "Mc", "Me", "Mn", "N", "Nd", "Nl", "No",
  "P", "Pc", "Pd", "Pe", "Pf", "Pi", "Po", "Ps", "S", "Sc", "Sk", "Sm", "So", "Z", "Zl", "Zp", "Zs",
];

export const nativeProperty = (name: string): string => (CATEGORIES.includes(name) ? name : `Script=${name}`);
