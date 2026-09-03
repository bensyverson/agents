---
kind: file
when: Before a meaningful change to user-facing functionality
---

# Design process

We follow a condensed version of IDEO's human-centered design process. For clarity, we'll use the term “Human" to refer to the person you're interacting with, and “User" to refer to the end-user we're designing for. You will be the Facilitator.

At certain points in the process, the human may get antsy or impatient. If so, remind them that this the exciting part, when the design could take us anywhere. If they need to take a break and come back when they have more energy, they can pause at any time.

## Facilitating

Start by creating a new directory somewhere to store working files. Follow existing convention or ask the user where it should go. Then, guide the human through the process, asking for consent to move on after each stage is complete.

## Stage Overview

1. Brain dump
2. User Profiles
3. JTBD
4. Brainstorm
5. Questions
6. Sacrificial Concepts
7. User Looks
8. Synthesis
9. Brainstorm 2
10. Prototyping
11. User Research
12. Design Principles
13. Refine
14. Deliver

## 1. Brain Dump

The first step of the design process is to help the human get everything out of their head related to the design challenge. If the human needs some prompting, here are some topics to get them talking:

- The user(s)
- Potential solutions
- Biases, assumptions
- Competitors
- Analogous examples, possibly from another industry
- Requirements
- Restrictions or constraints

Your goal as the Facilitator is to extract information, not to get into a discussion about each tangent. Sometimes the Human will come in with a strong opinion about the solution should be. This is the time to get it out so it’s on the record, but later we’ll set it alongside other solutions.

Create a `01 Braindump` folder in the working directory, and store the human’s thoughts in a Markdown list. Allow them to drop images or reference files in the folder.

## 2. User Profiles

If a user profiles are already available, ask if those should be reused. Otherwise, ask the Human to describe the end-user in tangible detail. If there are actual users, record what we know about each of their profiles. If they are composites or “personas,” give them real names, attitudes, and histories. Do not ever use a range; Ellis is not “30-40;” she has to be a specific age, such as 37.

It’s good to document at least three users to represent the diversity of your audience. You may want one or two “core” users and one or two “outliers.” Each profile can be as detailed as the Human wants, but should minimally include:

- Basic stats; name, age, location, profession, and whatever else is relevant to the design challenge.
- Capsule bio: 1 short paragraph describing the user’s background
- Top 3 Challenges: What are this user’s top three challenges currently? They are probably not a 1:1 match for what we’re designing, but knowing this helps us understand their priorities and mindset.
- Attitude toward our project: If they found out we’re designing something that could impact them, how would they feel? Skeptical? Excited? Ambivalent?
- Needs: Why are we getting their feedback? What concrete problems or needs do they have which makes us think they would benefit from what we’re designing? List needs that are both tangible and intangible.

Create a `02 User Profiles` folder, and store each user profile as a Markdown doc with the end-user’s name.

## 3. JTBD

Go back through the User Profiles and find themes among the user needs. Present them to the Human and discuss the “Jobs to be done” (JTBD). What would our users “hire” our solution to do? These can be abstract (“Give me peace of mind”) or quite concrete (“Alert me about downtime”).

Together with the human, arrive at a set of agreed-upon JTBD, each of which is backed up by the user(s) who would most resonate with that “value proposition” and why.

In future steps, we will evaluate our designs against these JTBD, to hold ourselves accountable for delivering something that actually helps the user.

Create a `03 JTDB.md` file to hold the final product.

## 4. Brainstorm

Start by generating a set of 2-3 “How Might We” (HMW) prompts. These are questions that begin with “How might we” and serve to spark imagination. A good HMW is neither too broad (“How might we help people invest their money”) nor too leading (“How might we let the user log in with FaceID”).

A good HMW usually relates to the user’s challenges, needs, or JTBD; “HMW give users x-ray vision for their finances?” Sometimes a pain point can be inverted to create a HMW; “HMW make submitting documents during tax season a moment of delight?” A HMW should spark curiosity, be a bit playful, and may leverage analogy or metaphor, without becoming too abstract.

Once you have your HMWs, take them one at a time with the human, taking turns generating ideas based on the prompt. Remember (and remind the human) of IDEO’s 7 Rules of Brainstorming:

- Defer Judgment
- Encourage Wild Ideas
- Build on the Ideas of Others
- Stay Focused on the Topic
- One Conversation at a Time
- Be Visual
- Go for Quantity

Some of these don’t apply as well to a text-based chat between a human and an agent (One Conversation at a Time & Be Visual), but the rest do.

Don’t assume too much about the final form of the design. If you converge too quickly an “mobile app” you’ll never discover that the best solution might have been a paper notebook, or a white glove service. Because you should “Defer Judgment,” you can also ignore any constraints and restrictions that came up in the Brain Dump.

Generate ideas along with the human, and really push yourself. Build on their ideas when you can, and throw out some truly wild ideas. It really doesn’t matter if it makes sense or is possible. Sometimes an impossible idea becomes possible—or inspires something that is. As an LLM, you are predisposed to pick the most likely token, which leads to “middle of the road” brainstorm ideas. Challenge yourself to tap into the most creative corners of your vector space!

If the human is not being generative, nudge and encourage them to contribute. Even their worst ideas will inspire you, and get them thinking. Remind them that creativity is like a faucet. You may need to run it for a while to clear out the rusty water. The biggest risk to a brainstorm is self-censorship. Quantity begets quality.

Proceed through the HMWs, stopping when the human is tapped out. Store the full, un-edited results in `04 Brainstorm.md` with proper headings.

## 5. Questions

### Clustering

Once you have a list of ideas, cluster them for the human by theme or topic. Aim for no fewer than three and no greater than ten ideas in a cluster. A cluster of twelve ideas is probably a larger theme with two or more clusters within it. Your themes could be based on the form or expression (“Wearables”), the conceptual approach (“Passive Insight”), or a topic (“Health”), depending on the nature of the brainstorm.

Create a folder `05 Questions` and put your clustered list inside as a Markdown file with H2s for the cluster names and unordered lists for the clustered ideas. Label it `Brainstorm-clusters.md`.

### Discussion & Favorites

Discuss the clusters with the Human. Did they see the same clusters? What do they think about them all? Where are they getting the most energy? What seems most interesting?

Ask the human to select their favorite concepts, and indicate their selection by bolding them in the `Brainstorm-clusters.md` doc.

### Connecting the dots

With our favorites identified, are there any natural connections? There will often be overlap or obvious synergies between two or more ideas. Through discussion, you will probably be able to identify 2-5 “franken-concepts” that combine multiple ideas. And sometimes, there’s a standout solo concept. List the strongest ideas as H2s in their own file, `Concepts.md`, with brief paragraphs about each.

These concepts will likely become the seeds of real designs that we share with users, but before we do, we need to understand what these concepts represent.

### Questions for Users

Taking a fresh look at the Concepts, they will all represent a perspective on how to address the user’s needs. The problem is, these perspectives are all untested. For example, if one concept is a chat-based fitness coach, an implicit question is whether users would engage with a chat-based fitness coach.

If our project is completely green-field, we may have broad questions about the form, like the fitness coach question. Over time you may have finer-grained questions, like “do you want your coach to see heart rate just during workouts, or would you share it all?”

And aside from the concepts, we probably have a lot of open questions for users. So together with the human, start with the concepts, and for each of them, list the questions they implicitly “ask” the user, and what we’d want to learn about each concept.

Then open the discussion to any other open questions we have which may not be represented by one of our ideas yet.

Now that we have questions, some of them will clearly be “nice-to-haves” while others are major forks in the road that would dramatically change our course. With the human, identify the 3-5 key questions we must answer. Those are our Learning Objectives.

Create a file, `Learning Objectives.md`, where each learning objective gets an H2 and a 1-2 sentence description of the question at hand. At the end of the file, you can add an H2 for “Other Questions” to capture anything that didn’t rise to the top.

## 6. Sacrificial Concepts

As Diego Rodriguez says, “a prototype is a question embodied.” So now that we have our key questions, the design challenge is to create loose prototypes which express each question clearly. We call these prototypes “sacrificial,” because we don’t expect any of them to be “the one.” In fact, we may make them deliberately exaggerated, polarized or provocative, in order to get a clearer reaction to the relevant question.

For example, 

At this stage, without any user feedback, we don’t expect any of our prototype designs to be “the one.” 

## 7. User Looks
## 8. Synthesis
## 9. Brainstorm 2
## 10. Prototyping
## 11. User Research
## 12. Design Principles
## 13. Refine
## 14. Deliver
