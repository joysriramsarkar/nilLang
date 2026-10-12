# Alap UI Component Catalog (U16.2, U7)

Generated from `pkg/nilui/components` (54 entries). This page is the
 human-readable companion to `docs/spec/UI_SPEC.md` §7, which stays the
contract source of truth.

## Foundation (6)

Page · Text · RichText · Box · Spacer · Divider

## Layout (11)

Column · Row · Stack · Grid · ScrollView · SafeArea · Padding · Align ·
SizedBox · Flex · Layout

## Input (11)

Button · IconButton · TextField · SearchField · PasswordField · Checkbox ·
CheckboxGroup · RadioGroup · Switch · Slider · Stepper

## Display / Data (9)

Image · Icon · List · LazyColumn · LazyRow · DataGrid · ProgressIndicator ·
ActivityIndicator · Badge

## Feedback (6)

Dialog · Modal · BottomSheet · Snackbar · Tooltip · Popover

## Navigation (6)

NavigationStack · AppBar · BottomNavigation · TabBar · PageView · Route

## Alap-special (5)

Canvas · Chart · SemanticGroup · FocusScope · AnimatedContainer

Each component guarantees: typed props/slots/modifiers/events, lifecycle
(mount/unmount), theme-aware rendering (light/dark), an a11y role + label,
and a mandatory test (`TestA11yGate_AllComponents` is the release gate).
Nilang binding: `cmd/nil` `convertHashToComponent` — unknown types emit
`UI0140` instead of dropping silently.
