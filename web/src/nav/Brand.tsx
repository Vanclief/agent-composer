import { Link } from "react-router-dom";

export function AgcMark() {
  return (
    <div className="builder-logo" aria-hidden="true">
      <svg
        width="14"
        height="14"
        viewBox="0 0 14 14"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="3.5" cy="3.5" r="1.6" />
        <circle cx="10.5" cy="7" r="1.6" />
        <circle cx="3.5" cy="10.5" r="1.6" />
        <path d="M5 4.2 L9 6.3 M5 9.8 L9 7.7" />
      </svg>
    </div>
  );
}

/** Brand — the left edge of every top bar. Navigation lives in the rail. */
export function TopbarBrand() {
  return (
    <Link to="/" className="builder-brand">
      <AgcMark />
      <b>agc</b>
    </Link>
  );
}
