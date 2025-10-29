import shutil
from pathlib import Path

def prompt_overwrite(path: Path, yes_all_flag: list) -> bool:
    if yes_all_flag[0]:
        return True
    while True:
        resp = input(f"(?) {path} already exists. Overwrite? (y/n/a=all): ").strip().lower()
        if resp in ("y", "yes"):
            return True
        elif resp in ("n", "no"):
            return False
        elif resp in ("a", "all"):
            yes_all_flag[0] = True
            return True
        else:
            print("Please enter 'y', 'n', or 'a' (all).")

# --- Source project root ---
SRC_DIR = Path(__file__).parent.resolve()

# --- Destination folder ---
DEST_DIR = Path(r"C:\HIVEMIND")
DEST_DIR.mkdir(parents=True, exist_ok=True)

# --- Files/folders actually needed for ping ---
FILES_TO_COPY = [
    "hvmd.exe",  # compiled Go binary
    ".env",      # environment file 
   # "scripts",   # batch scripts like pingDB.bat
]

yes_to_all = [False]

for item in FILES_TO_COPY:
    src_path = SRC_DIR / item
    dest_path = DEST_DIR / item

    if not src_path.exists():
        print(f"(!) Warning: {src_path} does not exist")
        continue

    if dest_path.exists():
        if not prompt_overwrite(dest_path, yes_to_all):
            print(f"(-) Skipping {dest_path}")
            continue
        if dest_path.is_dir():
            shutil.rmtree(dest_path)
        else:
            dest_path.unlink()

    if src_path.is_file():
        shutil.copy2(src_path, dest_path)
        print(f"(>) Copied file {src_path} -> {dest_path}")
    elif src_path.is_dir():
        shutil.copytree(src_path, dest_path)
        print(f"(>) Copied folder {src_path} -> {dest_path}")
