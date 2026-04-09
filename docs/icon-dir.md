# Supplying Your Own Icon Set

TF-Lens ships with a purpose-built SVG icon set that uses simple,
color-coded shapes. If you want to use the official AWS architecture
icons (or any other icon set), you can supply them via the `--icon-dir`
flag.

## Why TF-Lens Doesn't Bundle AWS Icons

AWS architecture icons are distributed under **CC-BY-ND 2.0 (No
Derivatives)**. Bundling and redistributing them inside an open-source
binary is legally ambiguous. The `--icon-dir` override lets *you* supply
the icons on your own machine without TF-Lens ever redistributing them.

## How to Use the Official AWS Icons

1. Download the AWS Architecture Icons from the official source:
   https://aws.amazon.com/architecture/icons/

2. Extract the archive. You'll find individual SVG files inside.

3. Rename the icons to match Terraform resource types (see below).

4. Run tf-lens with the `--icon-dir` flag:

```bash
tf-lens export --plan plan.json --icon-dir ~/.tf-lens/icons/ --out diagram.html
```

## Icon File Naming Convention

Icon files must be named exactly after the Terraform resource type they
represent, with a `.svg` extension:

| Terraform resource type       | Expected filename                  |
|-------------------------------|------------------------------------|
| `aws_instance`                | `aws_instance.svg`                 |
| `aws_lambda_function`         | `aws_lambda_function.svg`          |
| `aws_s3_bucket`               | `aws_s3_bucket.svg`                |
| `aws_db_instance`             | `aws_db_instance.svg`              |
| `aws_vpc`                     | `aws_vpc.svg`                      |

No mapping table is needed. If an exact match is not found, TF-Lens
tries prefix fallback (`aws_db_instance` → `aws_db.svg`) and finally
falls back to a generic dashed box.

## Sharing Diagrams With Custom Icons

When you export a diagram with `--icon-dir`, the SVG icons are embedded
inline in the HTML as base64 data URIs. Recipients do not need the icon
directory — the HTML file is fully self-contained.
