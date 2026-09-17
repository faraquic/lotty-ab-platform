const SEGMENT_COUNT = 6;
const ARC_DEG = 44;
const STEP_DEG = 360 / SEGMENT_COUNT;
const RADIUS = 26;
const CENTER = 32;

function polar(angleDeg: number): { x: number; y: number } {
  const angleRad = (angleDeg * Math.PI) / 180;
  return {
    x: CENTER + RADIUS * Math.cos(angleRad),
    y: CENTER + RADIUS * Math.sin(angleRad),
  };
}

function arcPath(startDeg: number): string {
  const start = polar(startDeg);
  const end = polar(startDeg + ARC_DEG);
  const fixed = (value: number): string => value.toFixed(2);
  const radius = String(RADIUS);
  return `M ${fixed(start.x)} ${fixed(start.y)} A ${radius} ${radius} 0 0 1 ${fixed(end.x)} ${fixed(end.y)}`;
}

const SEGMENTS: string[] = Array.from({ length: SEGMENT_COUNT }, (_, index) =>
  arcPath(-90 + index * STEP_DEG),
);

interface SegmentedSpinnerProps {
  size?: number | string;
  strokeWidth?: number;
  className?: string;
  'aria-label'?: string;
}

export function SegmentedSpinner({
  size = 24,
  strokeWidth = 5,
  className,
  'aria-label': ariaLabel,
}: SegmentedSpinnerProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 64 64"
      fill="none"
      className={className}
      {...(ariaLabel === undefined
        ? { 'aria-hidden': true as const }
        : { role: 'status' as const, 'aria-label': ariaLabel })}
    >
      <g
        className="segmented-spinner__group"
        fill="none"
        stroke="currentColor"
        strokeWidth={strokeWidth}
        strokeLinecap="round"
      >
        {SEGMENTS.map((d) => (
          <path key={d} d={d} />
        ))}
      </g>
    </svg>
  );
}
