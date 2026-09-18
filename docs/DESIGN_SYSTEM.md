# NIMNear — Design System

## Status

The NIMNear visual design is defined in the connected Figma Make file.

Figma Make is the visual source of truth. The confirmed dark-theme values remain the visual baseline. The light-theme values are semantic runtime counterparts added for the product theme switch and are not confirmed as native Figma Variables.

## Figma Workflow

When implementing a screen:
1. open the referenced Figma frame through Figma MCP;
2. inspect the frame structure;
3. inspect Figma variables;
4. identify reusable components;
5. inspect component variants;
6. inspect spacing and layout;
7. inspect typography;
8. inspect colors;
9. inspect states;
10. implement using the existing frontend stack.

Figma Make is the visual source of truth. Do not replace confirmed Figma decisions with defaults from the implementation library.

## shadcn/ui Relationship

shadcn/ui is an implementation foundation, not the visual source of truth.

Rules:
- Figma wins on visual design.
- shadcn/ui primitives may be restyled heavily to match the confirmed Figma values.
- Default shadcn radius, typography, spacing and colors are not authoritative.
- Use shadcn composition and accessibility behavior where helpful.
- Extract stable Figma decisions into CSS variables/tokens so shadcn components can share them consistently.

## Design Principles

NIMNear should be:
- mobile-first;
- clean;
- location/discovery focused;
- easy to scan;
- touch friendly;
- suitable for a Mini App/WebView.

Exact visual interpretation must come from Figma rather than these descriptive principles.

## Confirmed Visual System

### Colors

The following dark-theme values were confirmed in the Figma Make preview. They should be treated as runtime-observed CSS values, not as a native Figma Variable collection:

| Role | Value |
| --- | --- |
| background | `#0f0e10` |
| card | `#1a1920` |
| card-hover | `#201f29` |
| border | `#252430` |
| faint-border | `#3d3b4d` |
| muted-text | `#8b8a97` |
| text | `#ffffff` |
| accent | `#d7a6ff` |
| primary | `#8277ff` |
| focus-ring | `#a49cff` |
| avatar | `#5b5af7` |

Do not expand this into an invented token scale. Additional semantic colors are not confirmed here.

### Light Theme

The runtime light theme preserves the same semantic roles and switches them through `data-theme="light"` on the root element:

| Role | Value |
| --- | --- |
| background | `#f8f7fb` |
| card | `#ffffff` |
| card-hover | `#f0eef6` |
| border | `#dedbe8` |
| faint-border | `#c7c2d4` |
| muted-text | `#666272` |
| text | `#17151f` |
| accent | `#7445a4` |
| primary | `#675be7` |
| focus-ring | `#675be7` |

The user's explicit choice is stored in `localStorage`. Without a stored choice, the first render follows `prefers-color-scheme`.

### Typography

Runtime-observed in the visible Figma Make preview:

- Primary visible font: `Instrument Sans`.
- Observed sizes: `11`, `12`, `13`, `14`, `15`, `16`, `18`, `22`, `28`, and `42` px.
- Observed weights: `400`, `500`, `600`, and `700`.
- `DM Mono` exists in the Make stylesheet, but visible usage was not confirmed.
- Do not add a broader type scale until it is confirmed in Figma.

### Spacing

The following are runtime-observed layout measurements, not a complete spacing scale:

- Header height: approximately `53` px.
- Navigation controls use approximately `6px 12px` padding with a `4` px navigation gap.
- Larger event cards use `16` px padding and commonly `12` px internal gaps.
- Category cards use `16` px padding and commonly `12` px gaps.
- Compact city cards use `12` px padding and commonly `10` px gaps.
- Section heading spacing commonly includes `16` px below the heading.

Do not infer a full spacing scale from these observations.

### Radius

Runtime-observed values:

- Cards: `12` px.
- Common controls and navigation items: `8` px or `12` px.
- Pills and avatar controls: fully rounded/pill-shaped.

These observations do not establish a complete radius token scale.

### Shadows

No visible box shadows were observed in the inspected Figma Make states. Do not add elevation shadows unless a later Figma state confirms them.

## Card Styling

The inspected cards use the dark card surface, subtle border, and rounded corners. The confirmed card-hover value is `#201f29`. Event, category, city, and calendar cards follow this visual direction, with no visible shadow in the inspected states.

These are visual observations only. Figma Make did not expose native Figma component definitions or a component/variant inventory for this documentation pass.

## Icons

The inspected Figma Make preview uses Lucide icons. Confirmed visible icons include:

- Sparkles
- CalendarDays
- Compass
- Sun
- Bell
- ArrowRight

Use the matching Lucide icon when implementing these confirmed glyphs. Do not introduce a second icon system without a new Figma decision.

## Images

Respect image aspect ratios and crop behavior defined by Figma.

Use appropriate placeholders/fallbacks when remote content is unavailable.

## Accessibility

Maintain sufficient contrast.

Interactive controls must be usable by touch.

Use semantic controls where possible.

Visual fidelity must not require breaking basic accessibility.

## Updating This Document

When a Figma design becomes approved and stable, extract stable decisions into this file.

Do not document temporary experimentation as permanent design-system rules.

Keep runtime-observed values clearly labeled until native Figma Variables or named styles are explicitly exposed and confirmed.
