import type { CSSProperties, SVGProps } from "react"

interface LogoSupayProps
  extends Omit<SVGProps<SVGSVGElement>, "color"> {
  /**
   * Altura del logo.
   * Puede ser número (px) o cualquier unidad CSS.
   *
   * @example
   * size={40}
   * size="2.5rem"
   */
  size?: number | string

  /**
   * Color del logo.
   *
   * Usa currentColor por defecto para poder controlarlo
   * con Tailwind: text-primary, text-white, etc.
   */
  color?: string

  /**
   * Título accesible opcional.
   */
  title?: string
}

export function LogoSupay({
  size = 48,
  color = "currentColor",
  title,
  style,
  ...props
}: LogoSupayProps) {
  const logoStyle: CSSProperties = {
    display: "block",
    width: "auto",
    height: size,
    flexShrink: 0,
    ...style,
  }

  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="150 385 315 420"
      preserveAspectRatio="xMidYMid meet"
      fill="none"
      style={logoStyle}
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
      focusable="false"
      {...props}
    >
      {title && <title>{title}</title>}

      <g
        transform="translate(0, 1254) scale(0.1, -0.1)"
        fill={color}
        stroke="none"
      >
        <path d="M2918 8324 c-160 -163 -185 -198 -160 -229 4 -5 80 -81 168 -167 143 -141 164 -158 194 -158 30 0 51 17 202 167 167 167 186 193 160 228 -4 6 -79 81 -168 168 -147 144 -164 157 -197 157 -33 0 -48 -12 -199 -166z" />

        <path d="M2069 8063 c-118 -120 -225 -234 -236 -253 -12 -19 -31 -64 -43 -99 -21 -63 -21 -78 -21 -551 1 -417 3 -494 17 -541 43 -144 109 -206 374 -348 l205 -110 3 559 c1 308 6 560 10 560 12 0 310 -118 662 -263 168 -69 457 -186 644 -261 374 -151 455 -189 535 -249 l55 -40 38 44 c21 24 48 69 61 99 22 54 22 64 25 534 2 316 -1 497 -8 531 -13 65 -67 177 -110 230 -59 73 -333 375 -337 371 -3 -2 12 -62 31 -133 43 -157 60 -262 52 -323 -10 -73 -35 -135 -70 -176 l-34 -39 -71 50 c-39 28 -92 65 -117 82 -65 46 -116 58 -184 44 -66 -13 -87 -28 -278 -203 l-151 -138 -166 148 c-200 176 -222 192 -274 199 -68 9 -105 -5 -241 -93 l-131 -86 -25 23 c-39 37 -74 118 -81 191 -7 78 6 157 54 322 19 65 33 122 31 127 -2 6 -100 -88 -219 -208z" />

        <path d="M2713 6691 c-34 -3 -68 -12 -75 -18 -18 -19 -15 -79 7 -103 18 -20 28 -20 411 -20 392 0 393 0 414 22 25 27 26 69 1 99 l-19 24 -339 1 c-186 1 -366 -1 -400 -5z" />

        <path d="M3710 5817 c0 -715 -1 -743 -45 -829 -9 -18 -10 -28 -3 -28 25 0 446 278 495 327 58 59 108 153 123 232 16 86 12 454 -5 529 -25 107 -83 194 -177 267 -36 28 -362 185 -384 185 -2 0 -4 -307 -4 -683z" />

        <path d="M2651 6393 c-11 -10 -24 -32 -27 -50 -5 -26 -1 -37 19 -57 l26 -26 389 0 389 0 21 23 c28 29 28 67 1 101 l-20 26 -389 0 c-355 0 -390 -1 -409 -17z" />

        <path d="M1769 6242 c-2 -20 -2 -177 1 -348 l5 -310 37 -75 c64 -130 21 -99 880 -621 355 -215 385 -228 473 -208 59 13 212 106 278 169 96 91 139 239 107 371 -16 67 -79 160 -138 203 -26 19 -211 112 -411 206 -200 95 -531 255 -735 356 -317 157 -380 192 -433 239 l-61 56 -3 -38z" />

        <path d="M2663 6118 c-47 -23 -50 -114 -4 -134 9 -4 182 -8 385 -11 385 -4 415 -2 436 38 17 31 11 69 -15 94 l-24 25 -378 -1 c-247 0 -386 -4 -400 -11z" />
      </g>
    </svg>
  )
}