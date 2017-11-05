package main

import (
    "time"
)

type CLParameters struct {
    Login string
    StartDate time.Time
    EndDate time.Time
    JiraUrl string
    Debug bool
    DebugVerbose bool
}

type BoardCfg struct {
    Project string
    OpenStatus []string
    WipStatus []string
    IdleStatus []string
    DoneStatus []string
}

type Auth struct {
    Session struct {
        Name string `json:"name"`
        Value string `json:"value"`
    } `json:"session"`
    LoginInfo struct {
        FailedCount int16 `json:"failedCount"`
        LoginCount int16 `json:"loginCount"`
        LastFailedLoginTime string `json:"lastFailedLoginCount"`
        PreviousLoginTime string `json:"previousLoginTime"`
    }
}

type SearchResult struct {
    Expand string `json:"expand"`
    StartAt int `json:"startAt"`
    MaxResults int `json:"maxResults"`
    Total int `json:"total"`
    Issues []Issue `json:"issues"`
}

type Issue struct {
    Expand string `json:"expand"`
    ID string `json:"id"`
    Self string `json:"self"`
    Key string `json:"key"`
    Fields struct {
        Resolution interface{} `json:"resolution"`
        Lastviewed string `json:"lastViewed"`
        Aggregatetimeoriginalestimate interface{} `json:"aggregatetimeoriginalestimate"`
        Issuelinks []interface{} `json:"issuelinks"`
        Assignee struct {
            Self string `json:"self"`
            Name string `json:"name"`
            Key string `json:"key"`
            Emailaddress string `json:"emailAddress"`
            Avatarurls struct {
                Four8X48 string `json:"48x48"`
                Two4X24 string `json:"24x24"`
                One6X16 string `json:"16x16"`
                Three2X32 string `json:"32x32"`
            } `json:"avatarUrls"`
            Displayname string `json:"displayName"`
            Active bool `json:"active"`
            Timezone string `json:"timeZone"`
        } `json:"assignee"`
        Subtasks []interface{} `json:"subtasks"`
        Votes struct {
            Self string `json:"self"`
            Votes int `json:"votes"`
            Hasvoted bool `json:"hasVoted"`
        } `json:"votes"`
        Worklog struct {
            Startat int `json:"startAt"`
            Maxresults int `json:"maxResults"`
            Total int `json:"total"`
            Worklogs []interface{} `json:"worklogs"`
        } `json:"worklog"`
        Issuetype struct {
            Self string `json:"self"`
            ID string `json:"id"`
            Description string `json:"description"`
            Iconurl string `json:"iconUrl"`
            Name string `json:"name"`
            Subtask bool `json:"subtask"`
        } `json:"issuetype"`
        Timetracking struct {
        } `json:"timetracking"`
        Environment interface{} `json:"environment"`
        Duedate interface{} `json:"duedate"`
        Timeestimate interface{} `json:"timeestimate"`
        Status struct {
            Self string `json:"self"`
            Description string `json:"description"`
            Iconurl string `json:"iconUrl"`
            Name string `json:"name"`
            ID string `json:"id"`
            Statuscategory struct {
                Self string `json:"self"`
                ID int `json:"id"`
                Key string `json:"key"`
                Colorname string `json:"colorName"`
                Name string `json:"name"`
            } `json:"statusCategory"`
        } `json:"status"`
        Aggregatetimeestimate interface{} `json:"aggregatetimeestimate"`
        Creator struct {
            Self string `json:"self"`
            Name string `json:"name"`
            Key string `json:"key"`
            Emailaddress string `json:"emailAddress"`
            Avatarurls struct {
                Four8X48 string `json:"48x48"`
                Two4X24 string `json:"24x24"`
                One6X16 string `json:"16x16"`
                Three2X32 string `json:"32x32"`
            } `json:"avatarUrls"`
            Displayname string `json:"displayName"`
            Active bool `json:"active"`
            Timezone string `json:"timeZone"`
        } `json:"creator"`
        Timespent interface{} `json:"timespent"`
        Aggregatetimespent interface{} `json:"aggregatetimespent"`
        Workratio int `json:"workratio"`
        Labels []interface{} `json:"labels"`
        Components []interface{} `json:"components"`
        Reporter struct {
            Self string `json:"self"`
            Name string `json:"name"`
            Key string `json:"key"`
            Emailaddress string `json:"emailAddress"`
            Avatarurls struct {
                Four8X48 string `json:"48x48"`
                Two4X24 string `json:"24x24"`
                One6X16 string `json:"16x16"`
                Three2X32 string `json:"32x32"`
            } `json:"avatarUrls"`
            Displayname string `json:"displayName"`
            Active bool `json:"active"`
            Timezone string `json:"timeZone"`
        } `json:"reporter"`
        Progress struct {
            Progress int `json:"progress"`
            Total int `json:"total"`
        } `json:"progress"`
        Project struct {
            Self string `json:"self"`
            ID string `json:"id"`
            Key string `json:"key"`
            Name string `json:"name"`
            Avatarurls struct {
                Four8X48 string `json:"48x48"`
                Two4X24 string `json:"24x24"`
                One6X16 string `json:"16x16"`
                Three2X32 string `json:"32x32"`
            } `json:"avatarUrls"`
        } `json:"project"`
        Resolutiondate interface{} `json:"resolutiondate"`
        Watches struct {
            Self string `json:"self"`
            Watchcount int `json:"watchCount"`
            Iswatching bool `json:"isWatching"`
        } `json:"watches"`
        Updated string `json:"updated"`
        Timeoriginalestimate interface{} `json:"timeoriginalestimate"`
        Description string `json:"description"`
        Summary string `json:"summary"`
        Comment struct {
            Startat int `json:"startAt"`
            Maxresults int `json:"maxResults"`
            Total int `json:"total"`
            Comments []interface{} `json:"comments"`
        } `json:"comment"`
        Fixversions []interface{} `json:"fixVersions"`
        Priority struct {
            Self string `json:"self"`
            Iconurl string `json:"iconUrl"`
            Name string `json:"name"`
            ID string `json:"id"`
        } `json:"priority"`
        Versions []interface{} `json:"versions"`
        Aggregateprogress struct {
            Progress int `json:"progress"`
            Total int `json:"total"`
        } `json:"aggregateprogress"`
        Created string `json:"created"`
        Attachment []interface{} `json:"attachment"`
    } `json:"fields"`
    Changelog struct {
        Startat int `json:"startAt"`
        Maxresults int `json:"maxResults"`
        Total int `json:"total"`
        Histories []struct {
            ID string `json:"id"`
            Author struct {
                Self string `json:"self"`
                Name string `json:"name"`
                Key string `json:"key"`
                Emailaddress string `json:"emailAddress"`
                Avatarurls struct {
                    Four8X48 string `json:"48x48"`
                    Two4X24 string `json:"24x24"`
                    One6X16 string `json:"16x16"`
                    Three2X32 string `json:"32x32"`
                } `json:"avatarUrls"`
                Displayname string `json:"displayName"`
                Active bool `json:"active"`
                Timezone string `json:"timeZone"`
            } `json:"author"`
            Created string `json:"created"`
            Items []struct {
                Field string `json:"field"`
                Fieldtype string `json:"fieldtype"`
                From string `json:"from"`
                Fromstring string `json:"fromString"`
                To string `json:"to"`
                Tostring string `json:"toString"`
            } `json:"items"`
        } `json:"histories"`
    } `json:"changelog"`
}
