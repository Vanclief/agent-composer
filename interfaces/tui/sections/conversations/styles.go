package conversations

import "github.com/vanclief/agent-composer/interfaces/tui/sections/theme"

var (
	titleStyle   = theme.TitleStyle
	bodyStyle    = theme.BodyStyle
	statusStyle  = theme.BodyStyle
	loadingStyle = theme.LoadingStyle
	errorStyle   = theme.ErrorStyle
	labelStyle   = theme.BodyStyle.Bold(true)
	valueStyle   = theme.HighlightStyle
)
