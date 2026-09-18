package main

import (
	"time"

	"github.com/masterfabric-go/masterfabric/internal/application/event/dto"
)

const (
	catalogCity = "Istanbul"
	catalogTZ   = "Europe/Istanbul"
	lunaSmoke   = "0.00001"
)

// catalogEvent is one demo event. Theme and ImageSource are operator-facing
// labels only: the events table has no theme or attribution columns.
type catalogEvent struct {
	Title       string
	Theme       string
	Description string
	Address     string
	Latitude    float64
	Longitude   float64
	PriceNIM    string
	ImageURL    string
	ImageSource string
	DayOffset   int
	Hour        int
	Minute      int
	Duration    time.Duration
	Capacity    int
	SmokeTest   bool
}

func catalogEvents() []catalogEvent {
	return []catalogEvent{
		{
			Title: "Bosphorus Sunset Meetup",
			Theme: "Social / Lifestyle",
			Description: "An open evening on the Ortaköy waterfront as the sun drops behind the Bosphorus Bridge. Meet nearby at Ortaköy Mosque square, walk the promenade, and stay for the sunset light on the strait. Free to join; no ticket required.",
			Address:     "Ortaköy Meydanı, Mecidiye Mahallesi, Beşiktaş, Istanbul",
			Latitude:    41.04694,
			Longitude:   29.02694,
			PriceNIM:    "0",
			ImageURL:    unsplashPhoto("1763965368246-5e89755e4455"),
			ImageSource: "Unsplash, Ortaköy Mosque at sunset (O1n-f15L668)",
			DayOffset:   4,
			Hour:        18, Minute: 0, Duration: 2 * time.Hour, Capacity: 40,
		},
		{
			Title: "Historic Istanbul Photo Walk",
			Theme: "Photography / Culture",
			Description: "A slow morning walk through Sultanahmet with Hagia Sophia and the surrounding historic peninsula as the subject. Bring any camera. We meet at Sultanahmet Square and move on foot through the public streets. Free; limited group size so the walk stays quiet.",
			Address:     "Sultanahmet Square, Ayasofya Meydanı, Fatih, Istanbul",
			Latitude:    41.00833,
			Longitude:   28.98000,
			PriceNIM:    "0",
			ImageURL:    unsplashPhoto("1662633270606-96a958d1f79e"),
			ImageSource: "Unsplash, Hagia Sophia (eod3LcClIc8)",
			DayOffset:   8,
			Hour:        10, Minute: 0, Duration: 3 * time.Hour, Capacity: 20,
		},
		{
			Title: "Developer Community Meetup",
			Theme: "Technology / Community",
			Description: "A casual weekday meetup for Istanbul developers on the Kadıköy waterfront. Talks are short, the rest is conversation. Meet at Kadıköy Ferry Terminal and continue nearby in Caferağa. Free; bring a laptop only if you want to.",
			Address:     "Kadıköy Ferry Terminal, Damga Sokak, Caferağa, Kadıköy, Istanbul",
			Latitude:    40.9909,
			Longitude:   29.0187,
			PriceNIM:    "0",
			ImageURL:    unsplashPhoto("1667816144800-f46f30470d81"),
			ImageSource: "Unsplash, Kadıköy aerial (wFGPd_i5Zw4)",
			DayOffset:   11,
			Hour:        19, Minute: 0, Duration: 2 * time.Hour, Capacity: 50,
		},
		{
			Title: "Indie Music Night",
			Theme: "Music",
			Description: "A small-stage indie night in Kadıköy’s Caferağa side streets, around Kadife Sokak. Local bands, standing room, and a 1 Luna Mainnet smoke-test ticket so the paid checkout path can be verified with a real MainAlbatross payment.",
			Address:     "Caferağa Mahallesi, Kadıköy, Istanbul",
			Latitude:    40.993389,
			Longitude:   29.037361,
			PriceNIM:    lunaSmoke,
			ImageURL:    unsplashPhoto("1470229722913-7c0e2dbbafd3"),
			ImageSource: "Unsplash, live concert (photo-1470229722913-7c0e2dbbafd3)",
			DayOffset:   15,
			Hour:        21, Minute: 0, Duration: 3 * time.Hour, Capacity: 80,
			SmokeTest: true,
		},
		{
			Title: "Digital Art & Design Workshop",
			Theme: "Art / Design",
			Description: "An afternoon workshop in Karaköy for designers and illustrators. We work from the waterfront around Karaköy Pier and Kemankeş, looking at composition, type, and how the port streets read on a screen. Paid; bring a laptop or tablet.",
			Address:     "Karaköy Pier, Rıhtım Caddesi, Kemankeş Karamustafa Paşa, Beyoğlu, Istanbul",
			Latitude:    41.0217,
			Longitude:   28.9767,
			PriceNIM:    "2",
			ImageURL:    unsplashPhoto("1747156684442-831e095f97c2"),
			ImageSource: "Unsplash, Karaköy / Galata waterfront (IrklOUpVbE4)",
			DayOffset:   19,
			Hour:        14, Minute: 0, Duration: 3 * time.Hour, Capacity: 16,
		},
		{
			Title: "Istanbul Coffee Experience",
			Theme: "Food / Coffee",
			Description: "A tasting walk through the cafés around Galata Tower. We start at the tower, talk through Turkish coffee and contemporary espresso, and sit with a cup in Bereketzade. Paid; the ticket covers the guided tasting, not a full meal.",
			Address:     "Galata Kulesi, Bereketzade Mahallesi, Beyoğlu, Istanbul",
			Latitude:    41.0256556,
			Longitude:   28.9741556,
			PriceNIM:    "1",
			ImageURL:    unsplashPhoto("1757079649052-a24c6ab32c64"),
			ImageSource: "Unsplash, Turkish coffee in Istanbul (JNUELX786BU)",
			DayOffset:   22,
			Hour:        11, Minute: 0, Duration: 2 * time.Hour, Capacity: 12,
		},
		{
			Title: "Bosphorus Tech Networking Night",
			Theme: "Technology / Networking",
			Description: "An after-work gathering at Beşiktaş Square for engineers, operators, and product people. Short introductions, then open conversation looking out toward the Beşiktaş ferry and the Bosphorus. Paid; no pitch stage.",
			Address:     "Beşiktaş Meydanı, Sinanpaşa Mahallesi, Beşiktaş, Istanbul",
			Latitude:    41.04250,
			Longitude:   29.00722,
			PriceNIM:    "3",
			ImageURL:    unsplashPhoto("1763965367191-6455ef032c79"),
			ImageSource: "Unsplash, Istanbul skyline and Bosphorus (UM_YUJUGK6g)",
			DayOffset:   26,
			Hour:        19, Minute: 0, Duration: 150 * time.Minute, Capacity: 60,
		},
		{
			Title: "Street Photography Workshop",
			Theme: "Photography",
			Description: "A hands-on morning in Balat’s back streets. We meet on Vodina Caddesi among the painted houses and work on light, timing, and permission on a real residential street. Paid; any camera is welcome, including a phone.",
			Address:     "Vodina Caddesi, Balat, Fatih, Istanbul",
			Latitude:    41.0328,
			Longitude:   28.9456,
			PriceNIM:    "2.5",
			ImageURL:    unsplashPhoto("1612293509840-afb0f18d417b"),
			ImageSource: "Unsplash, Balat houses (c_Phj_lH7nQ)",
			DayOffset:   33,
			Hour:        10, Minute: 0, Duration: 3 * time.Hour, Capacity: 12,
		},
		{
			Title: "Startup Founders Meetup",
			Theme: "Business / Entrepreneurship",
			Description: "A founder-to-founder evening in Levent, next to Istanbul Sapphire on Büyükdere Caddesi. Small group, no demo day. Paid; intended for people currently building a company, not a public expo.",
			Address:     "Istanbul Sapphire, Büyükdere Caddesi, Levent, Beşiktaş, Istanbul",
			Latitude:    41.08500,
			Longitude:   29.00583,
			PriceNIM:    "1.5",
			ImageURL:    unsplashPhoto("1725181213139-1ccd6dbc9a44"),
			ImageSource: "Unsplash, Istanbul high-rise (qfpmnv4O-vQ)",
			DayOffset:   40,
			Hour:        18, Minute: 30, Duration: 2 * time.Hour, Capacity: 30,
		},
		{
			Title: "Istanbul Creative Night",
			Theme: "Creative / Design / Music",
			Description: "A late evening in Bomonti for designers, musicians, and independent studios. We gather at the historic Bomonti brewery campus in Şişli for sets, informal show-and-tell, and a long close. Paid.",
			Address:     "Tarihi Bomonti Bira Fabrikası, Birahane Sokak No:1, Şişli, Istanbul",
			Latitude:    41.05819,
			Longitude:   28.98075,
			PriceNIM:    "4",
			ImageURL:    unsplashPhoto("1514525253161-7a46d19cd819"),
			ImageSource: "Unsplash, night concert lights (photo-1514525253161-7a46d19cd819)",
			DayOffset:   47,
			Hour:        20, Minute: 0, Duration: 4 * time.Hour, Capacity: 100,
		},
	}
}

func unsplashPhoto(id string) string {
	return "https://images.unsplash.com/photo-" + id + "?auto=format&fit=crop&w=1600&q=80"
}

func (e catalogEvent) request(startsAt, endsAt time.Time) dto.CreateEventRequest {
	capacity := e.Capacity
	lat, lng := e.Latitude, e.Longitude
	address := e.Address
	return dto.CreateEventRequest{
		Title:       e.Title,
		Description: e.Description,
		StartsAt:    startsAt,
		EndsAt:      endsAt,
		PriceNIM:    e.PriceNIM,
		Currency:    "NIM",
		Capacity:    &capacity,
		ImageURL:    e.ImageURL,
		Latitude:    &lat,
		Longitude:   &lng,
		Address:     &address,
		City:        catalogCity,
	}
}

func (e catalogEvent) schedule(now time.Time, loc *time.Location) (time.Time, time.Time) {
	localNow := now.In(loc)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), e.Hour, e.Minute, 0, 0, loc).AddDate(0, 0, e.DayOffset)
	if !start.After(now) {
		start = start.AddDate(0, 0, 1)
	}
	return start.UTC(), start.Add(e.Duration).UTC()
}
