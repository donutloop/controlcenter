var insights = document.getElementById("insights")

insights.style.display = "none"

function fetchInsights() {
  var insightTemplate = document.getElementById("insight-template")

  var formData = new FormData()

  // TODO: use reasonable time interval :-)
  formData.append("from", "0000-01-01")
  formData.append("to", "3000-01-01")

  var filter = document.getElementById("insights-filter")

  var setCategoryFilter = function(category) {
      formData.append("filter", "category")
      formData.append("category", category)
  }

  var s = filter.value.split("-")

  if (s.length == 2 && s[0] == "category") {
    setCategoryFilter(s[1])
  }

  formData.append("entries", "6")

  formData.append("order", document.getElementById("insights-sort").value)

  var params = new URLSearchParams(formData)

  fetch("/api/v0/fetchInsights?" + params.toString()).then(res => res.json()).then(data => {
    if (data.length == 0) {
      insights.style.display = "none"
      return
    }

    insights.style.display = "flex"

    while (insights.firstChild) {
      insights.removeChild(insights.firstChild);
    }

    data.forEach(i => {
      var c = insightTemplate.cloneNode(true)
      c.querySelector(".time").innerHTML = buildInsightTime(i)
      c.querySelector(".title").innerHTML = buildInsightTitle(i)
      c.querySelector(".description").innerHTML = buildInsightDescription(i)
      insights.appendChild(c)
    })
  })
}

function buildInsightTime(insight) {
  return insight.Time
}

function buildInsightTitle(insight) {
  return insightsTitles[insight.ContentType]
}

function buildInsightDescription(insight) {
  var handler = insightsDescriptions[insight.ContentType]

  if (handler == undefined) {
    return "Description for " + insight.ContentType
  }

  return handler(insight.Content)
}
