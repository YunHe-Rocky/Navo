.PHONY: build build-launcher test vet package smoke clean

build:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/package.ps1

build-launcher:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/build.ps1

test:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test.ps1

vet:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/test.ps1 -VetOnly

package:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/package.ps1

smoke:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/smoke.ps1

clean:
	powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/clean.ps1
