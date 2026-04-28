declare const __BUILD_YEAR__: string;
declare const __PROJECT_VERSION__: string;

export function Footer() {
  return (
    <footer class="site-footer">
      <div class="site-footer-inner">
        <span>(c) 2025-{__BUILD_YEAR__} Asymmetric Effort, LLC. MIT License.</span>
        <span class="dot" aria-hidden="true">
          •
        </span>
        <span>{__PROJECT_VERSION__}</span>
        <span class="dot" aria-hidden="true">
          •
        </span>
        <a href="https://github.com/asymmetric-effort/convocate" target="_blank" rel="noopener noreferrer">
          Source
        </a>
        <span class="dot" aria-hidden="true">
          •
        </span>
        <a
          href="https://github.com/asymmetric-effort/convocate/issues"
          target="_blank"
          rel="noopener noreferrer"
        >
          Issues
        </a>
        <span class="dot" aria-hidden="true">
          •
        </span>
        <a
          href="https://github.com/asymmetric-effort/convocate/releases"
          target="_blank"
          rel="noopener noreferrer"
        >
          Releases
        </a>
      </div>
    </footer>
  );
}
