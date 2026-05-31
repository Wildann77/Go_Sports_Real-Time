package schemas

type ListMatchesQuery struct {
	Status string `form:"status"` // scheduled|live|finished
	Sport  string `form:"sport"`  // free text, ILIKE
	Team   string `form:"team"`   // search homeTeam OR awayTeam
	Page   int    `form:"page,default=1"`
	Limit  int    `form:"limit,default=25"`
	Sort   string `form:"sort"`   // date_asc|date_desc|status|team
}
