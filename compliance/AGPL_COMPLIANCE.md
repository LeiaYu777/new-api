# AGPL-3.0 SaaS Compliance Checklist

## 1. Source Offer
- Publish the exact running source (including your modifications) to customers using the hosted service.
- Keep build scripts, deployment manifests, and dependency lockfiles in the offered source bundle.

## 2. Copyright / License Notice
- Keep upstream copyright and AGPL license headers.
- Add your own copyright statement for modified files.
- Include `LICENSE` and this compliance note in customer delivery.

## 3. Change Log
- Maintain a human-readable change log for customer-facing releases.
- Link commit IDs or release tags to each change entry.

## 4. SBOM
- Generate an SBOM for every release:
  - `make sbom`
- Store SBOM artifacts with release packages.

## 5. Closed-source Commercial Path
- If closed-source delivery is required, obtain commercial authorization from the upstream rightsholder and retain:
  - Commercial contract
  - Invoice / payment proof
  - Authorized version scope and duration
