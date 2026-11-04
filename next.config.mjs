/** @type {import('next').NextConfig} */
export default {
  serverExternalPackages: ["better-sqlite3"],
  outputFileTracingIncludes: { "/**": ["./data/**", "./public/ScheherazadeNew-Regular.ttf"] },
};
