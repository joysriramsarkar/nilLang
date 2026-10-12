package main

// isCatalogType reports whether typ is one of the 54 U7 catalog components.
// The full catalog lives in pkg/nilui/components; this registry mirrors the
// names so the Nilang→Go bridge (U0.4) never silently drops a known type.
func isCatalogType(typ string) bool {
	switch typ {
	case "Page", "Text", "RichText", "Box", "Spacer", "Divider",
		"Column", "Row", "Stack", "Grid", "ScrollView", "SafeArea",
		"Padding", "Align", "SizedBox", "Flex", "Layout",
		"Button", "IconButton", "TextField", "SearchField", "PasswordField",
		"Checkbox", "CheckboxGroup", "RadioGroup", "Switch", "Slider", "Stepper",
		"Image", "Icon", "List", "LazyColumn", "LazyRow", "DataGrid",
		"ProgressIndicator", "ActivityIndicator", "Badge",
		"Dialog", "Modal", "BottomSheet", "Snackbar", "Tooltip", "Popover",
		"NavigationStack", "AppBar", "BottomNavigation", "TabBar", "PageView", "Route",
		"Canvas", "Chart", "SemanticGroup", "FocusScope", "AnimatedContainer",
		"Dashboard", "Table", "Form", "Card", "Tabs", "Navigation":
		return true
	}
	return false
}
