import contextlib
import importlib.metadata
import os
import sys

CHUNK = 16
TATWEEL = "ـ"
PACKAGES = ("camel-tools", "catt-tashkeel", "torch", "transformers", "onnxruntime", "numpy")


def versions():
    found = {}
    for name in PACKAGES:
        try:
            found[name] = importlib.metadata.version(name)
        except importlib.metadata.PackageNotFoundError:
            found[name] = None
    found["python"] = sys.version.split()[0]
    found["camel_data"] = os.environ.get("CAMELTOOLS_DATA", "")
    return found


class Engine:
    def __init__(self, log=None):
        self.log = log or (lambda message: None)
        self.analyzer = None
        self.bert = None
        self.catt = None
        self.dediac = None

    def load(self):
        if self.analyzer is not None:
            return
        self.log("loading the CAMeL morphology database")
        with contextlib.redirect_stdout(sys.stderr):
            from camel_tools.disambig.bert import BERTUnfactoredDisambiguator
            from camel_tools.morphology.analyzer import Analyzer
            from camel_tools.morphology.database import MorphologyDB
            from camel_tools.utils.dediac import dediac_ar

            self.dediac = dediac_ar
            self.analyzer = Analyzer(MorphologyDB.builtin_db())
            self.log("loading the CAMeL BERT disambiguator")
            self.bert = BERTUnfactoredDisambiguator.pretrained(use_gpu=False)
            self.log("loading CATT")
            from catt_tashkeel import CATTEncoderOnly

            self.catt = CATTEncoderOnly()

    def bare(self, tokens):
        return [self.dediac(t).replace(TATWEEL, "") for t in tokens]

    def check(self, items, on_reading, on_error, on_progress):
        self.load()
        cache = {}
        done = 0
        for start in range(0, len(items), CHUNK):
            chunk = items[start:start + CHUNK]
            bare = [self.bare(item["tokens"]) for item in chunk]
            vocalized = self.run_catt(chunk, bare)
            for item, words, catt in zip(chunk, bare, vocalized):
                try:
                    on_reading(self.reading(item, words, catt, cache))
                except Exception as exc:
                    on_error(item["key"], f"{type(exc).__name__}: {exc}")
                done += 1
            on_progress(done, len(items))

    def run_catt(self, chunk, bare):
        contextual = [i for i, item in enumerate(chunk) if item.get("context") and bare[i]]
        out = {}
        if contextual:
            try:
                with contextlib.redirect_stdout(sys.stderr):
                    sentences = [" ".join(bare[i]) for i in contextual]
                    results = self.catt.do_tashkeel_batch(sentences, verbose=False)
                out = dict(zip(contextual, results))
            except Exception as exc:
                self.log(f"CATT failed on a chunk: {type(exc).__name__}: {exc}")
        return [out.get(i) for i in range(len(chunk))]

    def reading(self, item, words, catt, cache):
        analyses = []
        for word in words:
            if word not in cache:
                cache[word] = sorted({a["diac"] for a in self.analyzer.analyze(word) if a.get("diac")})
            analyses.append(cache[word])
        catt_tokens = bert_tokens = None
        if item.get("context") and words:
            if catt is not None:
                split = catt.split()
                catt_tokens = split if len(split) == len(words) else None
            with contextlib.redirect_stdout(sys.stderr):
                disambiguated = self.bert.disambiguate(words)
            bert_tokens = [d.analyses[0].analysis.get("diac", "") if d.analyses else "" for d in disambiguated]
        return {"key": item["key"], "analyses": analyses, "catt": catt_tokens, "bert": bert_tokens}
