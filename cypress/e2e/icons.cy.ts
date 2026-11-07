export {};

const pngSize = (bytes: ArrayBuffer) => {
  const view = new DataView(bytes);
  return [view.getUint32(16), view.getUint32(20)];
};
const binary = (url: string) =>
  cy.window().then((win) =>
    win.fetch(url).then(async (res) => ({ status: res.status, type: res.headers.get("content-type"), body: await res.arrayBuffer() })),
  );
const crawler = { "User-Agent": "facebookexternalhit/1.1" };
const tag = (html: string, pattern: RegExp) => pattern.exec(html)?.[1];

describe("the icons and the share image", () => {
  it("links two icons and an Apple icon, and serves each with the right type and size", () => {
    cy.visit("/");
    cy.document().then((doc) => {
      const icons = Array.from(doc.querySelectorAll('link[rel="icon"]')).map((link) => link.getAttribute("href")?.split("?")[0]);
      expect(icons.sort()).to.deep.equal(["/favicon.ico", "/icon.svg"]);
      expect(doc.querySelector('link[rel="apple-touch-icon"]')?.getAttribute("sizes")).to.equal("180x180");
      const apple = doc.querySelector('link[rel="apple-touch-icon"]')!.getAttribute("href")!;
      binary(apple).then((res) => {
        expect(res.status, apple).to.equal(200);
        expect(res.type, apple).to.equal("image/png");
        expect(pngSize(res.body), apple).to.deep.equal([180, 180]);
      });
      const vector = doc.querySelector('link[rel="icon"][type="image/svg+xml"]')!.getAttribute("href")!;
      binary(vector).then((res) => {
        expect(res.status, vector).to.equal(200);
        expect(res.type, vector).to.equal("image/svg+xml");
        expect(new TextDecoder().decode(res.body.slice(0, 4)), vector).to.equal("<svg");
      });
      binary("/favicon.ico").then((res) => {
        const view = new DataView(res.body);
        expect(res.status).to.equal(200);
        expect(res.type).to.equal("image/x-icon");
        expect([view.getUint16(0, true), view.getUint16(2, true), view.getUint16(4, true)]).to.deep.equal([0, 1, 3]);
      });
    });
  });

  it("colours the browser's own chrome like the page, in light and in dark", () => {
    cy.visit("/");
    cy.document().then((doc) => {
      const colours = Array.from(doc.querySelectorAll('meta[name="theme-color"]')).map((meta) => `${meta.getAttribute("media")} ${meta.getAttribute("content")}`);
      expect(colours.sort()).to.deep.equal(["(prefers-color-scheme: dark) #191919", "(prefers-color-scheme: light) #ffffff"]);
    });
  });

  it("still answers a crawler that sends a Host header no address can be made from", () => {
    cy.request({ url: "/", headers: { ...crawler, Host: "a b" }, failOnStatusCode: false }).its("status").should("eq", 200);
  });

  it("gives a link preview its description and a 1200 by 630 image that resolves", () => {
    cy.request({ url: "/", headers: crawler }).then(({ body }) => {
      const html = body as string;
      expect(tag(html, /<meta property="og:description" content="([^"]*)"/)).to.equal("Build one Modern Standard Arabic Anki deck, one card at a time.");
      expect(tag(html, /<meta property="og:image:width" content="([^"]*)"/)).to.equal("1200");
      expect(tag(html, /<meta property="og:image:height" content="([^"]*)"/)).to.equal("630");
      expect(tag(html, /<meta property="og:image:alt" content="([^"]*)"/)).to.equal("Mufradat: build one Arabic Anki deck, one card at a time.");
      expect(tag(html, /<meta name="twitter:card" content="([^"]*)"/)).to.equal("summary_large_image");
      for (const pattern of [/<meta property="og:image" content="([^"]*)"/, /<meta name="twitter:image" content="([^"]*)"/]) {
        const url = tag(html, pattern)!;
        expect(url, "absolute URL on this host").to.match(new RegExp(`^${Cypress.config("baseUrl")}/`));
        cy.visit("/");
        binary(url).then((res) => {
          expect(res.status, url).to.equal(200);
          expect(res.type, url).to.equal("image/png");
          expect(pngSize(res.body), url).to.deep.equal([1200, 630]);
        });
      }
    });
  });
});
