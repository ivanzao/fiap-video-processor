#!/usr/bin/env python3
"""Valida os dashboards Grafana provisionados por infra/modules/observability/dashboards.tf.

Regras: JSON parseável com uid e title únicos; toda query filtra por $namespace; valores dos
labels status e outcome em UPPER_SNAKE, como os enums do domínio; ids de painel únicos.
"""
import json
import os
import re
import sys

DASHBOARD_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "infra", "modules", "observability", "dashboards")
ENUM_LABEL = re.compile(r'\b(status|outcome)\s*=~?\s*"([^"]+)"')


def queries(dashboard):
    for panel in dashboard.get("panels", []):
        for sub in [panel] + panel.get("panels", []):
            for target in sub.get("targets", []):
                expr = target.get("expr")
                if isinstance(expr, str) and expr.strip():
                    yield sub.get("title", "<sem titulo>"), expr


def validate(path):
    name = os.path.basename(path)
    try:
        with open(path, encoding="utf-8") as fh:
            dashboard = json.load(fh)
    except json.JSONDecodeError as exc:
        return [f"{name}: JSON inválido: {exc}"], None
    errors = []
    for field in ("uid", "title"):
        if not dashboard.get(field):
            errors.append(f"{name}: campo '{field}' ausente")
    ids = [p.get("id") for p in dashboard.get("panels", [])]
    if len(ids) != len(set(ids)):
        errors.append(f"{name}: ids de painel repetidos")
    for panel in dashboard.get("panels", []):
        for sub in [panel] + panel.get("panels", []):
            refs = [t.get("refId") for t in sub.get("targets", [])]
            if len(refs) != len(set(refs)):
                errors.append(f"{name} / {sub.get('title', '<sem titulo>')}: refId repetido entre queries")
    for title, expr in queries(dashboard):
        if "$namespace" not in expr and "vector(" not in expr:
            errors.append(f"{name} / {title}: query não filtra por $namespace")
        for label, value in ENUM_LABEL.findall(expr):
            for alternative in value.split("|"):
                bare = alternative.strip().strip(".*")
                if bare and re.fullmatch(r"[A-Za-z_]+", bare) and bare != bare.upper():
                    errors.append(f'{name} / {title}: {label}="{bare}" deveria ser UPPER_SNAKE')
    return errors, dashboard.get("uid")


def main():
    files = sorted(f for f in os.listdir(DASHBOARD_DIR) if f.endswith(".json"))
    if not files:
        print("nenhum dashboard encontrado", file=sys.stderr)
        return 1
    all_errors, uids = [], []
    for f in files:
        errors, uid = validate(os.path.join(DASHBOARD_DIR, f))
        all_errors += errors
        uids.append(uid)
    if len(uids) != len(set(uids)):
        all_errors.append("uids de dashboard repetidos")
    for e in all_errors:
        print(e, file=sys.stderr)
    print(f"{len(files)} dashboards, {len(all_errors)} problemas")
    return 1 if all_errors else 0


if __name__ == "__main__":
    sys.exit(main())
