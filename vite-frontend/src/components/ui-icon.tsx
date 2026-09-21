import React from "react";

import { IconSvgProps } from "@/types";

export type UiIconName =
  | "dashboard"
  | "server"
  | "protocol"
  | "relay"
  | "landing"
  | "users"
  | "limiter"
  | "tunnel"
  | "forward"
  | "settings"
  | "guide"
  | "menu"
  | "collapse"
  | "expand"
  | "key"
  | "link"
  | "refresh"
  | "zap"
  | "warning"
  | "info"
  | "check"
  | "x"
  | "globe"
  | "clipboard"
  | "plug"
  | "shield"
  | "lock"
  | "inbox"
  | "route"
  | "plus"
  | "edit"
  | "chevronDown"
  | "chevronUp";

type UiIconProps = IconSvgProps & { name: UiIconName };

const strokeProps = {
  fill: "none",
  stroke: "currentColor",
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  strokeWidth: 1.8,
};

const paths: Record<UiIconName, React.ReactNode> = {
  dashboard: <><rect {...strokeProps} x="3" y="3" width="7" height="7" rx="1" /><rect {...strokeProps} x="14" y="3" width="7" height="7" rx="1" /><rect {...strokeProps} x="3" y="14" width="7" height="7" rx="1" /><rect {...strokeProps} x="14" y="14" width="7" height="7" rx="1" /></>,
  server: <><rect {...strokeProps} x="3" y="3" width="18" height="7" rx="1.5" /><rect {...strokeProps} x="3" y="14" width="18" height="7" rx="1.5" /><path {...strokeProps} d="M7 6.5h.01M7 17.5h.01M11 6.5h6M11 17.5h6" /></>,
  protocol: <><path {...strokeProps} d="M12 3v18M3 8h18M3 16h18" /><circle {...strokeProps} cx="12" cy="12" r="9" /></>,
  relay: <><path {...strokeProps} d="M4 7h12M13 4l3 3-3 3M20 17H8M11 14l-3 3 3 3" /></>,
  landing: <><circle {...strokeProps} cx="12" cy="12" r="8.5" /><path {...strokeProps} d="M3.5 12h17M12 3.5c2.2 2.4 3.3 5.2 3.3 8.5S14.2 18.1 12 20.5C9.8 18.1 8.7 15.3 8.7 12S9.8 5.9 12 3.5z" /></>,
  users: <><path {...strokeProps} d="M16 20v-1.7a3.3 3.3 0 0 0-3.3-3.3H6.3A3.3 3.3 0 0 0 3 18.3V20" /><circle {...strokeProps} cx="9.5" cy="7" r="3.5" /><path {...strokeProps} d="M16 3.7a3.5 3.5 0 0 1 0 6.6M21 20v-1.7a3.3 3.3 0 0 0-2.5-3.2" /></>,
  limiter: <><path {...strokeProps} d="M4 19V9M10 19V5M16 19v-7M22 19H2" /><path {...strokeProps} d="M4 6l6-3 6 4 6-4" /></>,
  tunnel: <><path {...strokeProps} d="M4 7h16M4 17h16M7 4v6M17 14v6" /><circle {...strokeProps} cx="7" cy="7" r="2.5" /><circle {...strokeProps} cx="17" cy="17" r="2.5" /></>,
  forward: <><path {...strokeProps} d="M4 7h15M15 4l4 3-4 3M20 17H5M9 14l-4 3 4 3" /></>,
  settings: <><circle {...strokeProps} cx="12" cy="12" r="3" /><path {...strokeProps} d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-1.9 1.9-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5v.2h-2.7V20a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1-1.9-1.9.1-.1A1.7 1.7 0 0 0 7.7 15a1.7 1.7 0 0 0-1.5-1H6v-2.7h.2a1.7 1.7 0 0 0 1.5-1 1.7 1.7 0 0 0-.3-1.9l-.1-.1 1.9-1.9.1.1a1.7 1.7 0 0 0 1.9.3 1.7 1.7 0 0 0 1-1.5V5h2.7v.2a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1 1.9 1.9-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.5 1h.2V14h-.2a1.7 1.7 0 0 0-1.5 1z" /></>,
  guide: <><path {...strokeProps} d="M4 5.5A2.5 2.5 0 0 1 6.5 3H20v16H6.5A2.5 2.5 0 0 0 4 21V5.5z" /><path {...strokeProps} d="M4 5.5V21M8 7h8M8 11h8" /></>,
  menu: <><path {...strokeProps} d="M4 6h16M4 12h16M4 18h16" /></>,
  collapse: <><path {...strokeProps} d="M15 4l-8 8 8 8M7 12h13" /></>,
  expand: <><path {...strokeProps} d="M9 4l8 8-8 8M4 12h13" /></>,
  key: <><circle {...strokeProps} cx="8" cy="15" r="4" /><path {...strokeProps} d="M11 12l7-7M16 5l3 3M14 7l3 3" /></>,
  link: <><path {...strokeProps} d="M10 13.5a4 4 0 0 0 5.7.2l2.8-2.8a4 4 0 0 0-5.7-5.7l-1.6 1.6M14 10.5a4 4 0 0 0-5.7-.2l-2.8 2.8a4 4 0 0 0 5.7 5.7l1.6-1.6" /></>,
  refresh: <><path {...strokeProps} d="M20 11a8 8 0 0 0-13.6-5.7L4 7.7M4 4v3.7h3.7M4 13a8 8 0 0 0 13.6 5.7l2.4-2.4M20 20v-3.7h-3.7" /></>,
  zap: <><path {...strokeProps} d="M13 2L4 14h7l-1 8 9-12h-7l1-8z" /></>,
  warning: <><path {...strokeProps} d="M12 3l9 17H3L12 3z" /><path {...strokeProps} d="M12 9v4M12 16h.01" /></>,
  info: <><circle {...strokeProps} cx="12" cy="12" r="9" /><path {...strokeProps} d="M12 11v5M12 8h.01" /></>,
  check: <><circle {...strokeProps} cx="12" cy="12" r="9" /><path {...strokeProps} d="M8 12l2.5 2.5L16 9" /></>,
  x: <><circle {...strokeProps} cx="12" cy="12" r="9" /><path {...strokeProps} d="M9 9l6 6M15 9l-6 6" /></>,
  globe: <><circle {...strokeProps} cx="12" cy="12" r="9" /><path {...strokeProps} d="M3 12h18M12 3c2.4 2.6 3.5 5.6 3.5 9S14.4 18.4 12 21c-2.4-2.6-3.5-5.6-3.5-9S9.6 5.6 12 3z" /></>,
  clipboard: <><rect {...strokeProps} x="5" y="4" width="14" height="17" rx="1.5" /><path {...strokeProps} d="M9 4V3h6v1M8 9h8M8 13h8M8 17h5" /></>,
  plug: <><path {...strokeProps} d="M8 3v6M16 3v6M6 8h12v2a6 6 0 0 1-12 0V8zM12 16v5" /></>,
  shield: <><path {...strokeProps} d="M12 3l7 3v5c0 4.5-3 8-7 10-4-2-7-5.5-7-10V6l7-3z" /><path {...strokeProps} d="M9 12l2 2 4-4" /></>,
  lock: <><rect {...strokeProps} x="5" y="10" width="14" height="11" rx="2" /><path {...strokeProps} d="M8 10V7a4 4 0 0 1 8 0v3M12 14v3" /></>,
  inbox: <><path {...strokeProps} d="M4 5h16l2 10H2L4 5zM2 15v3a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-3M8 12h8" /></>,
  plus: <><path {...strokeProps} d="M12 5v14M5 12h14" /></>,
  edit: <><path {...strokeProps} d="M4 20h4L19 9l-4-4L4 16v4zM13.5 6.5l4 4" /></>,
  chevronDown: <><path {...strokeProps} d="M6 9l6 6 6-6" /></>,
  chevronUp: <><path {...strokeProps} d="M6 15l6-6 6 6" /></>,
  route: <><circle {...strokeProps} cx="5" cy="6" r="2" /><circle {...strokeProps} cx="19" cy="18" r="2" /><path {...strokeProps} d="M7 6h5a4 4 0 0 1 4 4v2a4 4 0 0 0 4 4h-1" /></>,
};

export function UiIcon({ name, size = 18, width, height, ...props }: UiIconProps) {
  return (
    <svg
      aria-hidden={props["aria-label"] ? undefined : true}
      focusable="false"
      height={size || height}
      role="img"
      viewBox="0 0 24 24"
      width={size || width}
      {...props}
    >
      {paths[name]}
    </svg>
  );
}
