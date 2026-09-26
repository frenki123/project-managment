# CAD Development APP
This is basic design document how we need to develop simple project managment app that can replace Excel file `CAD Development 2026_new.xlsx`

# Original Excel
This Excel separates planning, execution, and progress into three linked sheets (`Plan`, `Actual`, `Progress`) so that estimated hours, real hours spent, and task completion can
be tracked independently instead of assuming "hours used = progress made." Each task's `Complexity` (h), set in `Plan`, is turned into a weight factor in `Progress` 
(task complexity / total complexity of all tasks) and combined with weekly progress % to produce a cumulative earned-value figure in hours — putting progress on the same scale as
planned and actual hours. These three hour-based curves (Planned, Actual, Earned) are plotted together on the `S curve` sheet, which is the key output of the whole file: 
a real picture of whether the project is on track, even when time spent doesn't match work actually done.

The key point of this Excel is that we can have estimated hours of 100, developer hours of 100, and progress can still be 50%. This just means project progress isn't going as 
planned, and that's exactly what the S-curve is meant to visualize — unlike other project management tools that would automatically mark a task as finished once all the hours are 
used. Planned hours, used hours, and progress must stay conceptually separate, and `Complexity` is what lets progress (%) be converted into the same hours metric as planned/actual,
so all three can be plotted together.

## Excel Data Model
This is list of used data in the Excel. It's not divided by sheet as sheet usage was just for the Excel, limitiation in the design not something we want to copy.
In general we don't care on which sheet is something, but just what data represents. Basic data for this are list of tasks with static data and weekly hours and progress tracking.

*User Inputs*:
- task ID
- task name
- task description
- task implementation notes
- relevent department
- developer/developers
- priority
- weekly estimation of the planed hours for the task
- weekly actual hours worked per taks
- weekly progress estimation per task. Progress can't go down so if progeress is 50% in W31 and we didn't make any progress in W32, progress in W32 is 50%. So progress is cumulative

*Calculated results*
- Complexity (Total Hours per Task)
- Total planned hours per week
- Total planned hours cumulative - so we can track how much total hours is planed for project
- Total hours spent/actual per Task
- Total hours spent/actual per week
- Total hours spent/actual per week cumulative
- Task progress per week - max value entered for progress
- Task weight factor - ratio between task and total complexity `TotalTaskHours / SumOfTotalTaskHours`
- Total weekly progress - sumproduct between task weight factor and task progress
- S-curve graph

*Mising data*
- Project concept - in Excel we simplified the problem with project per year, but this is wrong. In the app we need to be able to define tasks and projects. Important is that we can have
tasks that don't have project. These tasks represents idea, but currently are not assigned on any project.



